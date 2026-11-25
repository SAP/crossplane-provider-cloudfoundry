package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"k8s.io/utils/ptr"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
)

const (
	// RestartAnnotation requests a rolling restart; each new non-empty value triggers one.
	RestartAnnotation = "app.cloudfoundry.crossplane.io/restart"

	// RestartStampAnnotation holds the hex SHA-256 of the restart value on the CF
	// deployment, so the provider finds its own deployment after a lost status save.
	RestartStampAnnotation = "crossplane-restart-sha256"

	// DeploymentStrategyRolling is the CF rolling deployment strategy.
	DeploymentStrategyRolling = "rolling"

	// CF deployment status values.
	DeploymentStateActive    = "ACTIVE"
	DeploymentStateFinalized = "FINALIZED"

	// DeploymentReasonDeployed is the CF status reason of a successful deployment.
	DeploymentReasonDeployed = "DEPLOYED"
	// DeploymentReasonNotFound marks a tracked deployment that no longer exists in CF.
	DeploymentReasonNotFound = "NOT_FOUND"
)

// DeploymentClient defines the interface to communicate with Cloud Foundry deployments.
type DeploymentClient interface {
	Create(ctx context.Context, r *resource.DeploymentCreate) (*resource.Deployment, error)
	Get(ctx context.Context, guid string) (*resource.Deployment, error)
	List(ctx context.Context, opts *client.DeploymentListOptions) ([]*resource.Deployment, *client.Pager, error)
}

// PendingRestart returns the restart annotation value if it is set and not handled yet.
func PendingRestart(cr *v1alpha1.App) (string, bool) {
	value := cr.GetAnnotations()[RestartAnnotation]
	if value == "" || value == cr.Status.AtProvider.LastHandledRestart {
		return "", false
	}
	return value, true
}

// RestartStamp returns the hex SHA-256 of a restart value, so any value fits CF metadata limits.
func RestartStamp(restart string) string {
	sum := sha256.Sum256([]byte(restart))
	return hex.EncodeToString(sum[:])
}

// IsRestartDeployment reports whether the deployment was started for the restart value.
func IsRestartDeployment(d *resource.Deployment, restart string) bool {
	if d.Metadata == nil {
		return false
	}
	stamp := d.Metadata.Annotations[RestartStampAnnotation]
	return stamp != nil && *stamp == RestartStamp(restart)
}

// NewDeploymentObservation maps a CF deployment to its observation.
// restart is the restart value that started it, empty if unknown.
func NewDeploymentObservation(d *resource.Deployment, restart string) *v1alpha1.AppDeploymentObservation {
	o := &v1alpha1.AppDeploymentObservation{
		GUID:      d.GUID,
		Strategy:  d.Strategy,
		State:     d.Status.Value,
		Reason:    d.Status.Reason,
		CreatedAt: ptr.To(d.CreatedAt.Format(time.RFC3339)),
		Restart:   restart,
	}
	if details := d.Status.Details; details != nil {
		if details.LastSuccessfulHealthcheck != nil {
			o.LastSuccessfulHealthcheck = ptr.To(details.LastSuccessfulHealthcheck.Format(time.RFC3339))
		}
		o.Error = details.Error
	}
	return o
}

// DeploymentActive reports whether the observed deployment is still running.
func DeploymentActive(o *v1alpha1.AppDeploymentObservation) bool {
	return o != nil && o.State == DeploymentStateActive
}

// LatestDeployment returns the app's newest deployment in any state, or nil.
// CF supersedes the active deployment on create, so it is always the newest.
func (c *Client) LatestDeployment(ctx context.Context, appGUID string) (*resource.Deployment, error) {
	opts := client.NewDeploymentListOptions()
	opts.AppGUIDs = client.Filter{Values: []string{appGUID}}
	opts.OrderBy = "-created_at"
	opts.PerPage = 1
	deployments, _, err := c.Deployments.List(ctx, opts)
	if err != nil || len(deployments) == 0 {
		return nil, err
	}
	return deployments[0], nil
}

// StartRollingRestart creates a rolling deployment of the app's current
// droplet, stamped with the restart value.
func (c *Client) StartRollingRestart(ctx context.Context, appGUID, restart string) (*resource.Deployment, error) {
	create := resource.NewDeploymentCreate(appGUID)
	create.Strategy = DeploymentStrategyRolling
	create.Metadata = resource.NewMetadata()
	create.Metadata.SetAnnotation("", RestartStampAnnotation, RestartStamp(restart))
	return c.Deployments.Create(ctx, create)
}
