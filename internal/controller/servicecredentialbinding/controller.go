package servicecredentialbinding

import (
	"context"
	"errors"
	"fmt"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	ctrl "sigs.k8s.io/controller-runtime"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	apisv1beta1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/v1beta1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients"
	scb "github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/servicecredentialbinding"

	cfclient "github.com/cloudfoundry/go-cfclient/v3/client"
	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	resourceType         = "ServiceCredentialBinding"
	externalSystem       = "Cloud Foundry"
	errTrackPCUsage      = "cannot track ProviderConfig usage: %w"
	errNewClient         = "cannot create a client for " + externalSystem + ": %w"
	errWrongCRType       = "managed resource is not a " + resourceType
	errGet               = "cannot get " + resourceType + " in " + externalSystem + ": %w"
	errFind              = "cannot find " + resourceType + " in " + externalSystem
	errCreate            = "cannot create " + resourceType + " in " + externalSystem + ": %w"
	errUpdate            = "cannot update " + resourceType + " in " + externalSystem + ": %w"
	errDelete            = "cannot delete " + resourceType + " in " + externalSystem + ": %w"
	errDeleteRetiredKeys = "cannot delete retired keys in " + externalSystem + ": %w"
	errDeleteExpiredKeys = "cannot delete expired keys in " + externalSystem + ": %w"
	errUpdateStatus      = "cannot update status after retiring binding: %w"
	errExtractParams     = "cannot extract specified parameters: %w"
	errUnknownState      = "unknown last operation state for " + resourceType + " in " + externalSystem
)

// Setup adds a controller that reconciles ServiceCredentialBinding CR.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.ServiceCredentialBindingGroupKind)

	options := []managed.ReconcilerOption{
		managed.WithInitializers(),
		managed.WithExternalConnecter(&connector{
			kube:  mgr.GetClient(),
			usage: resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1beta1.ProviderConfigUsage{}),
		}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithPollInterval(o.PollInterval),
	}


	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.ServiceCredentialBindingGroupVersionKind),
		options...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&v1alpha1.ServiceCredentialBinding{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector is expected to produce an external client when its Connect method
// is called.
type connector struct {
	kube  k8s.Client
	usage *resource.ProviderConfigUsageTracker
}

// Connect typically produces an ExternalClient by:
// 1. Tracking that the managed resource is using a ProviderConfig.
// 2. Getting the managed resource's ProviderConfig.
// 3. Getting the credentials specified by the ProviderConfig.
// 4. Using the credentials to form a client.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	if _, ok := mg.(*v1alpha1.ServiceCredentialBinding); !ok {
		return nil, errors.New(errWrongCRType)
	}

	if err := c.usage.Track(ctx, mg.(resource.ModernManaged)); err != nil {
		return nil, fmt.Errorf(errTrackPCUsage, err)
	}

	cf, err := clients.ClientFnBuilder(ctx, c.kube)(mg)
	if err != nil {
		return nil, fmt.Errorf(errNewClient, err)
	}

	client := scb.NewClient(cf)
	ext := &external{
		kube:      c.kube,
		scbClient: client,
		keyRotator: &scb.SCBKeyRotator{
			SCBClient: client,
		},
	}
	ext.observationStateHandler = ext // Use self as the default handler
	return ext, nil
}

// Disconnect implements the managed.ExternalClient interface
func (c *external) Disconnect(ctx context.Context) error {
	// No cleanup needed for Cloud Foundry client
	return nil
}

// ObservationStateHandler defines the interface for handling observation state
type ObservationStateHandler interface {
	HandleObservationState(serviceBinding *cfresource.ServiceCredentialBinding, ctx context.Context, cr *v1alpha1.ServiceCredentialBinding) (managed.ExternalObservation, error)
}

// An external service observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	kube                    k8s.Client
	scbClient               scb.ServiceCredentialBinding
	keyRotator              scb.KeyRotator
	observationStateHandler ObservationStateHandler
}

// Observe checks the observed state of the resource and updates the managed resource's status.
func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.ServiceCredentialBinding)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errWrongCRType)
	}

	guid := meta.GetExternalName(cr)
	serviceBinding, err := scb.GetByIDOrSearch(ctx, c.scbClient, guid, cr.Spec.ForProvider)
	if errors.Is(err, cfclient.ErrNoResultsReturned) ||
		errors.Is(err, cfclient.ErrExactlyOneResultNotReturned) ||
		cfresource.IsResourceNotFoundError(err) ||
		cfresource.IsServiceBindingNotFoundError(err) {
		return managed.ExternalObservation{ResourceExists: false}, nil
	} else if err != nil {
		return managed.ExternalObservation{}, fmt.Errorf(errGet, err)
	}

	cr.Status.AtProvider.GUID = serviceBinding.GUID
	cr.Status.AtProvider.CreatedAt = &metav1.Time{Time: serviceBinding.CreatedAt}

	if c.keyRotator.RetireBinding(cr, serviceBinding) {
		if err := c.kube.Status().Update(ctx, cr); err != nil {
			return managed.ExternalObservation{}, fmt.Errorf(errUpdateStatus, err)
		}
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	scb.UpdateObservation(&cr.Status.AtProvider, serviceBinding)

	return c.observationStateHandler.HandleObservationState(serviceBinding, ctx, cr)
}

// Create a ServiceCredentialBinding resource.
func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.ServiceCredentialBinding)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errWrongCRType)
	}

	params, err := extractParameters(ctx, c.kube, cr.Spec.ForProvider)
	if err != nil {
		return managed.ExternalCreation{}, fmt.Errorf(errExtractParams, err)
	}

	serviceBinding, err := scb.Create(ctx, c.scbClient, cr.Spec.ForProvider, params)
	if err != nil {
		return managed.ExternalCreation{}, fmt.Errorf(errCreate, err)
	}

	meta.SetExternalName(cr, serviceBinding.GUID)

	if cr.ObjectMeta.Annotations != nil {
		if _, ok := cr.ObjectMeta.Annotations[scb.ForceRotationKey]; ok {
			meta.RemoveAnnotations(cr, scb.ForceRotationKey)
		}
	}

	return managed.ExternalCreation{}, nil
}

// Update a ServiceCredentialBinding resource.
func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.ServiceCredentialBinding)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errWrongCRType)
	}

	if externalName := meta.GetExternalName(cr); externalName != "" {
		if _, err := scb.Update(ctx, c.scbClient, meta.GetExternalName(cr), cr.Spec.ForProvider); err != nil {
			return managed.ExternalUpdate{}, fmt.Errorf(errUpdate, err)
		}
	}

	if cr.Status.AtProvider.RetiredKeys == nil {
		return managed.ExternalUpdate{}, nil
	}

	if newRetiredKeys, err := c.keyRotator.DeleteExpiredKeys(ctx, cr); err != nil {
		return managed.ExternalUpdate{}, fmt.Errorf(errDeleteExpiredKeys, err)
	} else {
		cr.Status.AtProvider.RetiredKeys = newRetiredKeys
		return managed.ExternalUpdate{}, err
	}
}

// Delete a ServiceCredentialBinding resource.
func (c *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1alpha1.ServiceCredentialBinding)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errWrongCRType)
	}
	cr.SetConditions(xpv1.Deleting())

	if err := c.keyRotator.DeleteRetiredKeys(ctx, cr); err != nil {
		return managed.ExternalDelete{}, fmt.Errorf(errDeleteRetiredKeys, err)
	}

	err := scb.Delete(ctx, c.scbClient, cr.GetID())
	if err != nil {
		return managed.ExternalDelete{}, fmt.Errorf(errDelete, err)
	}

	return managed.ExternalDelete{}, nil
}

// extractParameters returns the parameters or credentials from the spec
func extractParameters(ctx context.Context, kube k8s.Client, spec v1alpha1.ServiceCredentialBindingParameters) ([]byte, error) {
	// If the spec has yaml parameters use those and only those.
	if spec.Parameters != nil {
		return spec.Parameters.Raw, nil
	}

	if spec.ParametersSecretRef != nil {
		return clients.ExtractSecret(ctx, kube, spec.ParametersSecretRef, "")
	}

	// If the spec has no parameters or secret ref, return nil
	return nil, nil
}

func (c *external) HandleObservationState(serviceBinding *cfresource.ServiceCredentialBinding, ctx context.Context, cr *v1alpha1.ServiceCredentialBinding) (managed.ExternalObservation, error) {
	switch serviceBinding.LastOperation.State {
	case v1alpha1.LastOperationInitial, v1alpha1.LastOperationInProgress:
		cr.SetConditions(xpv1.Unavailable().WithMessage(serviceBinding.LastOperation.Description))
		return managed.ExternalObservation{
			ResourceExists:   true,
			ResourceUpToDate: true, // Do not update the resource while the last operation is in progress
		}, nil
	case v1alpha1.LastOperationFailed:
		cr.SetConditions(xpv1.Unavailable().WithMessage(serviceBinding.LastOperation.Description))
		return managed.ExternalObservation{
			ResourceExists:   serviceBinding.LastOperation.Type != v1alpha1.LastOperationCreate, // set to false when the last operation is create, hence the reconciler will retry create
			ResourceUpToDate: serviceBinding.LastOperation.Type != v1alpha1.LastOperationUpdate, // set to false when the last operation is update, hence the reconciler will retry update
		}, nil
	case v1alpha1.LastOperationSucceeded:
		cr.SetConditions(xpv1.Available())

		return managed.ExternalObservation{
			ResourceExists:    true,
			ResourceUpToDate:  scb.IsUpToDate(ctx, cr.Spec.ForProvider, *serviceBinding) && !c.keyRotator.HasExpiredKeys(cr),
			ConnectionDetails: scb.GetConnectionDetails(ctx, c.scbClient, serviceBinding.GUID, cr.Spec.ConnectionDetailsAsJSON),
		}, nil
	}

	// If the last operation is unknown, error out
	return managed.ExternalObservation{}, errors.New(errUnknownState)
}
