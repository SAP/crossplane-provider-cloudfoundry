package domain

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"k8s.io/utils/ptr"

	ctrl "sigs.k8s.io/controller-runtime"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	pcv1beta1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/v1beta1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients"
	domain "github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/domain"
)

const (
	resourceType         = "Domain"
	externalSystem       = "Cloud Foundry"
	errNotDomainKind     = "managed resource is not of kind " + resourceType
	errNameRequired      = "name is required, please set the name attribute"
	errTrackUsage        = "cannot track usage"
	errGetProviderConfig = "cannot get ProviderConfig or resolve credential references"
	errGetClient         = "cannot create a client to talk to the API of" + externalSystem
	errGetResource       = "cannot get " + externalSystem + " domain according to the specified parameters"
	errCreate            = "cannot create " + externalSystem + " domain"
	errGet               = "cannot get " + resourceType + " in " + externalSystem
	errDelete            = "cannot delete" + resourceType
	errUpdate            = "cannot update" + resourceType
)

// Setup adds a controller that reconciles Org resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.Domain_GroupKind)

	options := []managed.ReconcilerOption{

		managed.WithExternalConnecter(&connector{
			kube:  mgr.GetClient(),
			usage: resource.NewProviderConfigUsageTracker(mgr.GetClient(), &pcv1beta1.ProviderConfigUsage{}),
		}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithInitializers(initializer{
			client: mgr.GetClient(),
		}),
	}


	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.Domain_GroupVersionKind),
		options...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&v1alpha1.Domain{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector supplies a function for the Reconciler to create a client to the external CloudFoundry resources.
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
	if _, ok := mg.(*v1alpha1.Domain); !ok {
		return nil, errors.New(errNotDomainKind)
	}

	if err := c.usage.Track(ctx, mg.(resource.ModernManaged)); err != nil {
		return nil, errors.Wrap(err, errTrackUsage)
	}

	cf, err := clients.ClientFnBuilder(ctx, c.kube)(mg)
	if err != nil {
		return nil, errors.Wrap(err, errGetClient)
	}

	return &external{client: domain.NewClient(cf), kube: c.kube}, nil
}

// Disconnect implements the managed.ExternalClient interface
func (c *external) Disconnect(ctx context.Context) error {
	// No cleanup needed for Cloud Foundry client
	return nil
}

// An external is a managed.ExternalConnecter that is using the CloudFoundry API to observe and modify resources.
type external struct {
	client domain.Client
	kube   k8s.Client
}

// Observe managed resource Domain
func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.Domain)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotDomainKind)
	}

	domainID := meta.GetExternalName(cr)

	d, err := domain.GetByIDOrName(ctx, c.client, domainID, cr.Spec.ForProvider.Name)

	if err != nil {
		if clients.ErrorIsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}

		return managed.ExternalObservation{}, errors.Wrap(err, errGetResource)
	}

	resourceLateInitialized := domain.LateInitialize()

	// set the external name to the GUID
	if domainID != d.GUID {
		meta.SetExternalName(cr, d.GUID)
		resourceLateInitialized = true
	}

	cr.SetConditions(xpv1.Available())

	cr.Status.AtProvider = domain.GenerateObservation(d)

	return managed.ExternalObservation{
		ResourceExists:          cr.Status.AtProvider.ID != nil,
		ResourceUpToDate:        domain.IsUpToDate(cr.Spec.ForProvider, d),
		ResourceLateInitialized: resourceLateInitialized,
	}, nil
}

// Create a managed resource Domain
func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.Domain)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotDomainKind)
	}

	cr.SetConditions(xpv1.Creating())

	o, err := c.client.Create(ctx, domain.GenerateCreate(cr.Spec.ForProvider))
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreate)
	}

	meta.SetExternalName(cr, o.GUID)

	return managed.ExternalCreation{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

// Update managed resource Domain
func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.Domain)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotDomainKind)
	}

	// assert that ID is set
	if cr.Status.AtProvider.ID == nil {
		return managed.ExternalUpdate{}, errors.New(errUpdate)
	}

	// rename resource
	if cr.Name != ptr.Deref(cr.Status.AtProvider.Name, "") {
		_, err := c.client.Update(ctx, *cr.Status.AtProvider.ID, domain.GenerateUpdate(cr.Spec.ForProvider))
		if err != nil {
			return managed.ExternalUpdate{}, errors.Wrap(err, errUpdate)
		}
	}

	return managed.ExternalUpdate{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

// Delete managed resource Domain
func (c *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1alpha1.Domain)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotDomainKind)
	}
	cr.SetConditions(xpv1.Deleting())

	// assert that ID is set
	if cr.Status.AtProvider.ID == nil {
		return managed.ExternalDelete{}, errors.New(errDelete)
	}

	_, err := c.client.Delete(ctx, *cr.Status.AtProvider.ID)
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, errDelete)
	}
	return managed.ExternalDelete{}, nil
}

// initializer type implements the managed.Initializer interface
type initializer struct {
	client k8s.Reader
}

// Initialize method resolves the references which are not resolved by
// the crossplane reconciler.
func (i initializer) Initialize(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.Domain)
	if !ok {
		return errors.New(errNotDomainKind)
	}

	// check if the name is already set
	if cr.Spec.ForProvider.Name == "" {
		// if name is not set, calculate name by domain and subdomain
		if cr.Spec.ForProvider.SubDomain == nil || cr.Spec.ForProvider.Domain == nil {
			return errors.New(errNameRequired) // if subdomain is not set
		}

		cr.Spec.ForProvider.Name = fmt.Sprintf("%s.%s", *cr.Spec.ForProvider.SubDomain, *cr.Spec.ForProvider.Domain)
	}
	return nil

}
