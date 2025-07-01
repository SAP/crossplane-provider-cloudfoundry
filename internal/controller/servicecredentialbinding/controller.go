package servicecredentialbinding

import (
	"context"
	"time"

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

	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	serviceBinding, err := scb.GetByIDOrSearch(ctx, c.scbClient, guid, *cr)
	if err != nil {
		if err.Error() == scb.ErrNameMissing || cfresource.IsResourceNotFoundError(err) || cfresource.IsServiceBindingNotFoundError(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errGet)
	}

	if serviceBinding == nil {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	if retireBinding(cr, serviceBinding) {
		if err := c.kube.Status().Update(ctx, cr); err != nil {
			return managed.ExternalObservation{}, errors.Wrap(err, "cannot update status after retiring binding")
		}
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
			ResourceUpToDate:  scb.IsUpToDate(ctx, cr.Spec.ForProvider, *serviceBinding) && !hasExpiredKeys(cr),
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

	if externalName := meta.GetExternalName(cr); externalName != "" {
		keyRetired := false
		for _, retiredKey := range cr.Status.AtProvider.RetiredKeys {
			if retiredKey.GUID == externalName {
				keyRetired = true
				break
			}
		}
		if !keyRetired {
			return managed.ExternalCreation{}, errors.New("cannot create a new ServiceCredentialBinding before retiring the existing one")
		}
	}

	params, err := extractParameters(ctx, c.kube, cr.Spec.ForProvider)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "cannot extract specified parameters")
	}
	cr.SetConditions(xpv1.Creating())

	serviceBinding, err := scb.Create(ctx, c.scbClient, *cr, params)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreate)
	}

	cr.Status.AtProvider.Name = *serviceBinding.Name
	cr.Status.AtProvider.GUID = serviceBinding.GUID
	cr.Status.AtProvider.CreatedAt = &metav1.Time{Time: serviceBinding.CreatedAt}

	if err := c.kube.Status().Update(ctx, cr); err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "cannot update status after creating service credential binding")
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

	if externalName := meta.GetExternalName(cr); externalName != "" {
		_, err := scb.Update(ctx, c.scbClient, meta.GetExternalName(cr), cr.Spec.ForProvider)
		if err != nil {
			return managed.ExternalUpdate{}, errors.Wrap(err, errUpdate)
		}
	}

	if cr.Status.AtProvider.RetiredKeys == nil {
		return managed.ExternalUpdate{}, nil
	}

	var newRetiredKeys []*v1alpha1.SCBResource
	var retireError error

	for _, key := range cr.Status.AtProvider.RetiredKeys {

		if key.CreatedAt.Add(cr.Spec.ForProvider.Rotation.TTL.Duration).After(time.Now()) {
			newRetiredKeys = append(newRetiredKeys, key)
		} else {
			if err := scb.Delete(ctx, c.scbClient, key.GUID); err != nil {
				if cfresource.IsResourceNotFoundError(err) || cfresource.IsServiceBindingNotFoundError(err) {
					continue // If the key is already deleted, we can ignore the error
				}
				newRetiredKeys = append(newRetiredKeys, key) // If we cannot delete the key, keep it in the list
				retireError = errors.Wrapf(err, "cannot delete retired key %s", key.GUID)
			}
		}
	}

	cr.Status.AtProvider.RetiredKeys = newRetiredKeys
	if err := c.kube.Status().Update(ctx, cr); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, "cannot update status after deleting retired keys")
	}

	return managed.ExternalUpdate{}, retireError
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

	for _, retiredKey := range cr.Status.AtProvider.RetiredKeys {
		if err := scb.Delete(ctx, c.scbClient, retiredKey.GUID); err != nil {
			if cfresource.IsResourceNotFoundError(err) || cfresource.IsServiceBindingNotFoundError(err) {
				continue
			}
			return errors.Wrapf(err, "cannot delete retired key %s", retiredKey.GUID)
		}
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

func retireBinding(cr *v1alpha1.ServiceCredentialBinding, serviceBinding *cfresource.ServiceCredentialBinding) bool {
	if cr.Spec.ForProvider.Rotation == nil {
		return false
	}

	if cr.Status.AtProvider.CreatedAt == nil || cr.Status.AtProvider.CreatedAt.Add(cr.Spec.ForProvider.Rotation.Frequency.Duration).Before(time.Now()) {
		// If the binding was created before the rotation frequency, retire it.
		for _, retiredKey := range cr.Status.AtProvider.RetiredKeys {
			if retiredKey.GUID == serviceBinding.GUID {
				// If the binding is already retired, do not retire it again.
				return true
			}
		}
		cr.Status.AtProvider.RetiredKeys = append(cr.Status.AtProvider.RetiredKeys, &v1alpha1.SCBResource{
			GUID:      serviceBinding.GUID,
			CreatedAt: &metav1.Time{Time: serviceBinding.CreatedAt},
		})
		return true
	}

	return false
}

func hasExpiredKeys(cr *v1alpha1.ServiceCredentialBinding) bool {
	if cr.Status.AtProvider.RetiredKeys == nil {
		return false
	}

	for _, key := range cr.Status.AtProvider.RetiredKeys {
		if key.CreatedAt.Add(cr.Spec.ForProvider.Rotation.TTL.Duration).Before(time.Now()) {
			return true
		}
	}

	return false
}
