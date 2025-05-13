package servicecredentialbinding

import (
	"context"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/connection"
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	apisv1alpha1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/v1alpha1"
	apisv1beta1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/v1beta1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients"
	scb "github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/servicecredentialbinding"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/features"
)

const (
	resourceType    = "ServiceCredentialBinding"
	externalSystem  = "Cloud Foundry"
	errTrackPCUsage = "cannot track ProviderConfig usage"
	errNewClient    = "cannot create a client for " + externalSystem
	errWrongCRType  = "managed resource is not a " + resourceType
	errGet          = "cannot get " + resourceType + " in " + externalSystem
	errFind         = "cannot find " + resourceType + " in " + externalSystem
	errCreate       = "cannot create " + resourceType + " in " + externalSystem
	errUpdate       = "cannot update " + resourceType + " in " + externalSystem
	errDelete       = "cannot delete " + resourceType + " in " + externalSystem
)

// Setup adds a controller that reconciles ServiceCredentialBinding CR.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.ServiceCredentialBindingGroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
	if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), apisv1alpha1.StoreConfigGroupVersionKind))
	}

	options := []managed.ReconcilerOption{
		managed.WithInitializers(),
		managed.WithExternalConnecter(&connector{
			kube:  mgr.GetClient(),
			usage: resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1beta1.ProviderConfigUsage{}),
		}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithConnectionPublishers(cps...),
		managed.WithPollInterval(o.PollInterval),
	}

	if o.Features.Enabled(features.EnableBetaManagementPolicies) {
		options = append(options, managed.WithManagementPolicies())
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
	usage resource.Tracker
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

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	cf, err := clients.ClientFnBuilder(ctx, c.kube)(mg)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &external{
		kube:      c.kube,
		scbClient: scb.NewClient(cf),
	}, nil
}

// An external service observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	kube      k8s.Client
	scbClient scb.ServiceCredentialBinding
}

// Observe checks the observed state of the resource and updates the managed resource's status.
func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.ServiceCredentialBinding)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errWrongCRType)
	}

	guid := meta.GetExternalName(cr)
	serviceBinding, err := scb.GetByIDOrSearch(ctx, c.scbClient, guid, cr.Spec.ForProvider)
	if err != nil {
		if clients.ErrorIsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errGet)
	}

	if serviceBinding == nil {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	// Update the external name if it is different from the GUID
	if guid != serviceBinding.GUID {
		meta.SetExternalName(cr, serviceBinding.Resource.GUID)
		if err := c.kube.Update(ctx, cr); err != nil {
			return managed.ExternalObservation{ResourceExists: true}, err
		}
	}

	scb.UpdateObservation(&cr.Status.AtProvider, serviceBinding)

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
			ResourceUpToDate:  scb.IsUpToDate(ctx, cr.Spec.ForProvider, *serviceBinding),
			ConnectionDetails: scb.GetConnectionDetails(ctx, c.scbClient, serviceBinding.GUID, cr.Spec.ConnectionDetailsAsJSON),
		}, nil
	}

	// If the last operation is unknown, error out
	return managed.ExternalObservation{}, errors.New("unknown last operation state")
}

// Create a ServiceCredentialBinding resource.
func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.ServiceCredentialBinding)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errWrongCRType)
	}

	params, err := extractParameters(ctx, c.kube, cr.Spec.ForProvider)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "cannot extract specified parameters")
	}
	cr.SetConditions(xpv1.Creating())

	serviceBinding, err := scb.Create(ctx, c.scbClient, cr.Spec.ForProvider, params)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreate)
	}

	meta.SetExternalName(cr, serviceBinding.GUID)

	return managed.ExternalCreation{}, nil
}

// Update a ServiceCredentialBinding resource.
func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.ServiceCredentialBinding)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errWrongCRType)
	}

	_, err := scb.Update(ctx, c.scbClient, cr.GetID(), cr.Spec.ForProvider)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdate)
	}

	return managed.ExternalUpdate{}, nil
}

// Delete a ServiceCredentialBinding resource.
func (c *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.ServiceCredentialBinding)
	if !ok {
		return errors.New(errWrongCRType)
	}
	cr.SetConditions(xpv1.Deleting())

	err := scb.Delete(ctx, c.scbClient, cr.GetID())
	if err != nil {
		return errors.Wrap(err, errDelete)
	}

	return nil
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
