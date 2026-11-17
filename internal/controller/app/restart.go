package app

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/pkg/errors"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/app"
)

const (
	reasonDeploymentStarted   event.Reason = "DeploymentStarted"
	reasonDeploymentSucceeded event.Reason = "DeploymentSucceeded"
	reasonDeploymentFailed    event.Reason = "DeploymentFailed"
	reasonRestartSkipped      event.Reason = "RestartSkipped"
	reasonRestartDeferred     event.Reason = "RestartDeferred"
)

var (
	errObserveDeployment = "Cannot observe Cloud Foundry deployment of " + resourceKind
	errListDeployments   = "Cannot list Cloud Foundry deployments of " + resourceKind
	errCreateDeployment  = "Cannot create rolling Cloud Foundry deployment of " + resourceKind
	errPersistStatus     = "Cannot persist restart status of " + resourceKind
)

// emit records an event; tests may build an external without a recorder.
func (c *external) emit(cr *v1alpha1.App, e event.Event) {
	if c.recorder == nil {
		return
	}
	c.recorder.Event(cr, e)
}

// deploymentUpToDate holds back spec changes while a deployment runs and
// reports a pending restart as out of date.
func (c *external) deploymentUpToDate(ctx context.Context, cr *v1alpha1.App, specUpToDate bool) (bool, error) {
	if err := c.observeDeployment(ctx, cr); err != nil {
		return false, err
	}
	if app.DeploymentActive(cr.Status.AtProvider.Deployment) {
		return true, nil
	}
	if _, pending := app.PendingRestart(cr); pending {
		return false, nil
	}
	return specUpToDate, nil
}

// observeDeployment refreshes the tracked deployment while it is active and
// records a deleted one as NOT_FOUND.
func (c *external) observeDeployment(ctx context.Context, cr *v1alpha1.App) error {
	prev := cr.Status.AtProvider.Deployment
	if !app.DeploymentActive(prev) {
		return nil
	}

	d, err := c.client.Deployments.Get(ctx, prev.GUID)
	var next *v1alpha1.AppDeploymentObservation
	switch {
	case clients.ErrorIsNotFound(err):
		next = prev.DeepCopy()
		next.State, next.Reason = app.DeploymentStateFinalized, app.DeploymentReasonNotFound
	case err != nil:
		return errors.Wrap(err, errObserveDeployment)
	default:
		next = app.NewDeploymentObservation(d, prev.Restart)
	}
	if app.DeploymentActive(next) {
		cr.Status.AtProvider.Deployment = next
		return nil
	}
	return c.saveThenEmit(ctx, cr, next, cr.Status.AtProvider.LastHandledRestart, finalEvent(next))
}

// finalEvent reports the outcome of a final deployment.
func finalEvent(d *v1alpha1.AppDeploymentObservation) event.Event {
	if d.Reason == app.DeploymentReasonDeployed {
		return event.Normal(reasonDeploymentSucceeded, fmt.Sprintf("%s deployment %s finished", d.Strategy, d.GUID))
	}
	return event.Warning(reasonDeploymentFailed, fmt.Errorf("%s deployment %s ended with reason %s", d.Strategy, d.GUID, d.Reason))
}

// saveThenEmit saves d and lastHandled, then emits e. A failed save is undone
// so the runtime's write-back after the error cannot drop the event.
func (c *external) saveThenEmit(ctx context.Context, cr *v1alpha1.App, d *v1alpha1.AppDeploymentObservation, lastHandled string, e event.Event) error {
	at := &cr.Status.AtProvider
	prevDeployment, prevLastHandled := at.Deployment, at.LastHandledRestart
	at.Deployment, at.LastHandledRestart = d, lastHandled
	if err := c.persistStatus(ctx, cr); err != nil {
		at.Deployment, at.LastHandledRestart = prevDeployment, prevLastHandled
		return err
	}
	c.emit(cr, e)
	return nil
}

// recoverRestart adopts the app's latest deployment if it carries this
// restart's stamp but isn't tracked (status save lost after Create), and waits
// for any other active deployment. handled=true means Update must return.
func (c *external) recoverRestart(ctx context.Context, cr *v1alpha1.App, restart string) (bool, error) {
	d, err := c.client.LatestDeployment(ctx, meta.GetExternalName(cr))
	if err != nil {
		return false, errors.Wrap(err, errListDeployments)
	}
	if d == nil {
		return false, nil
	}
	tracked := cr.Status.AtProvider.Deployment
	if app.IsRestartDeployment(d, restart) && (tracked == nil || tracked.GUID != d.GUID) {
		adopted := app.NewDeploymentObservation(d, restart)
		if !app.DeploymentActive(adopted) {
			return true, c.saveThenEmit(ctx, cr, adopted, restart, finalEvent(adopted))
		}
		// Kept on a failed save: no event depends on it.
		cr.Status.AtProvider.Deployment = adopted
		cr.Status.AtProvider.LastHandledRestart = restart
		return true, c.persistStatus(ctx, cr)
	}
	if d.Status.Value != app.DeploymentStateActive {
		return false, nil
	}
	return true, c.saveThenEmit(ctx, cr, app.NewDeploymentObservation(d, ""), cr.Status.AtProvider.LastHandledRestart,
		event.Normal(reasonRestartDeferred, fmt.Sprintf("restart waits for active deployment %s", d.GUID)))
}

// startRestart starts a rolling deployment of the current droplet. A stopped
// app is skipped, because CF would start every instance.
func (c *external) startRestart(ctx context.Context, cr *v1alpha1.App, restart string) error {
	if cr.Status.AtProvider.State == "STOPPED" {
		return c.saveThenEmit(ctx, cr, cr.Status.AtProvider.Deployment, restart,
			event.Normal(reasonRestartSkipped, "app is stopped, rolling restart skipped"))
	}

	d, err := c.client.StartRollingRestart(ctx, meta.GetExternalName(cr), restart)
	if err != nil {
		return errors.Wrap(err, errCreateDeployment)
	}
	// Kept even if the save fails: undoing it could start a second deployment,
	// and recoverRestart adopts it without a second event.
	c.emit(cr, event.Normal(reasonDeploymentStarted, fmt.Sprintf("rolling deployment %s started", d.GUID)))
	cr.Status.AtProvider.Deployment = app.NewDeploymentObservation(d, restart)
	cr.Status.AtProvider.LastHandledRestart = restart
	return c.persistStatus(ctx, cr)
}

// persistStatus saves status right away, so a failed reconciler write after
// Update cannot lose the deployment record.
func (c *external) persistStatus(ctx context.Context, cr *v1alpha1.App) error {
	return errors.Wrap(c.kube.Status().Update(ctx, cr), errPersistStatus)
}
