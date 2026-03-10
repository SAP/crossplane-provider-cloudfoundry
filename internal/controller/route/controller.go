package route

import (
	"context"

	"github.com/pkg/errors"

	ctrl "sigs.k8s.io/controller-runtime"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reference"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources"
	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	apisv1beta1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/v1beta1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/domain"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/route"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/space"
)

type RouteService interface {
	GetByIDOrSpec(ctx context.Context, guid string, forProvider v1alpha1.RouteParameters) (*v1alpha1.RouteObservation, error)
	Create(ctx context.Context, forProvider v1alpha1.RouteParameters) (string, error)
	Update(ctx context.Context, guid string, forProvider v1alpha1.RouteParameters) error
	Delete(ctx context.Context, guid string) error
}

const (
	errTrackPCUsage  = "cannot track ProviderConfig usage"
	errGetPC         = "cannot get ProviderConfig"
	errGetCreds      = "cannot get credentials"
	errNewClient     = "cannot create new client"
	errNotRoute      = "managed resource is not a cloudfoundry Route"
	errGet           = "cannot get cloudfoundry Route"
	errCreate        = "cannot create cloudfoundry Route"
	errUpdate        = "cannot update cloudfoundry Route"
	errDelete        = "cannot delete cloudfoundry Route"
	errActiveBinding = "cannot delete route with active bindings. Please remove the bindings first."
)

// Setup adds a controller that reconciles Org managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.RouteGroupKind)

	options := []managed.ReconcilerOption{
		managed.WithInitializers(
			domainInitializer{client: mgr.GetClient()},
			spaceInitializer{client: mgr.GetClient()},
		),
		managed.WithExternalConnecter(&connector{
			kube:  mgr.GetClient(),
			usage: resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1beta1.ProviderConfigUsage{}),
		}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithPollInterval(o.PollInterval),
	}


	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.RouteGroupVersionKind),
		options...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&v1alpha1.Route{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector is expected to produce an ExternalClient when its Connect method
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
	if _, ok := mg.(*v1alpha1.Route); !ok {
		return nil, errors.New(errNotRoute)
	}

	if err := c.usage.Track(ctx, mg.(resource.ModernManaged)); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	cf, err := clients.ClientFnBuilder(ctx, c.kube)(mg)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &external{RouteService: route.NewClient(cf), kube: c.kube}, nil
}

// Disconnect implements the managed.ExternalClient interface
func (c *external) Disconnect(ctx context.Context) error {
	// No cleanup needed for Cloud Foundry client
	return nil
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	// A 'client' used to connect to the external resource API. In practice this
	// would be something like an AWS SDK client.
	kube k8s.Client
	RouteService
}

// Observe generates observation for Route's
func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.Route)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotRoute)
	}

	guid := meta.GetExternalName(cr)

	atProvider, err := c.RouteService.GetByIDOrSpec(ctx, guid, cr.Spec.ForProvider)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errGet)
	}

	if atProvider == nil {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	cr.SetConditions(xpv1.Available())

	lateInitialized := false
	if atProvider.Resource.GUID != guid {
		meta.SetExternalName(cr, atProvider.Resource.GUID)
		lateInitialized = true
	}

	cr.Status.AtProvider = *atProvider

	return managed.ExternalObservation{
		ResourceExists:          true,
		ResourceUpToDate:        route.IsUpToDate(cr.Spec.ForProvider, *atProvider),
		ResourceLateInitialized: lateInitialized,
	}, nil

}

// Create a route
func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.Route)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotRoute)
	}

	cr.SetConditions(xpv1.Creating())

	guid, err := c.RouteService.Create(ctx, cr.Spec.ForProvider)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreate)
	}

	meta.SetExternalName(cr, guid)

	return managed.ExternalCreation{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

// Update updates a route
func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.Route)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotRoute)
	}

	guid := meta.GetExternalName(cr)
	err := c.RouteService.Update(ctx, guid, cr.Spec.ForProvider)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdate)
	}

	return managed.ExternalUpdate{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

// Delete deletes a route
func (c *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1alpha1.Route)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotRoute)
	}

	// Prevent delete if there are bindings.
	if len(cr.Status.AtProvider.Destinations) > 0 {
		return managed.ExternalDelete{}, errors.New(errActiveBinding)
	}

	cr.SetConditions(xpv1.Deleting())

	return managed.ExternalDelete{}, c.RouteService.Delete(ctx, meta.GetExternalName(cr))

}

// ResolveReferences of this Route.
func ResolveReferences(ctx context.Context, mg *v1alpha1.Route, c k8s.Reader) error {
	r := reference.NewAPIResolver(c, mg)

	var rsp reference.ResolutionResponse
	var err error

	rsp, err = r.Resolve(ctx, reference.ResolutionRequest{
		CurrentValue: reference.FromPtrValue(mg.Spec.ForProvider.Domain),
		Extract:      resources.ExternalID(),
		Reference:    clients.NamespacedRefToRef(mg.Spec.ForProvider.DomainRef),
		Selector:     clients.NamespacedSelectorToSelector(mg.Spec.ForProvider.DomainSelector),
		Namespace:    mg.GetNamespace(),
		To: reference.To{
			List:    &v1alpha1.DomainList{},
			Managed: &v1alpha1.Domain{},
		},
	})
	if err != nil {
		return errors.Wrap(err, "mg.Spec.ForProvider.Domain")
	}
	mg.Spec.ForProvider.Domain = reference.ToPtrValue(rsp.ResolvedValue)
	mg.Spec.ForProvider.DomainRef = clients.RefToNamespacedRef(rsp.ResolvedReference)

	return nil
}

// initializer type implements the managed.Initializer interface
type initializer struct {
	client k8s.Client
}

type domainInitializer initializer

// Initialize method resolves the references which are not resolved by
// the crossplane reconciler.
func (i domainInitializer) Initialize(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.Route)
	if !ok {
		return errors.New(errNotRoute)
	}

	if cr.Spec.ForProvider.DomainRef != nil || cr.Spec.ForProvider.DomainSelector != nil {
		return ResolveReferences(ctx, cr, i.client)
	}
	return domain.ResolveByName(ctx, clients.ClientFnBuilder(ctx, i.client), mg)
}

type spaceInitializer initializer

func (s spaceInitializer) Initialize(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.Route)
	if !ok {
		return errors.New(errNotRoute)
	}

	if cr.Spec.ForProvider.SpaceRef != nil || cr.Spec.ForProvider.SpaceSelector != nil {
		return cr.ResolveReferences(ctx, s.client)
	}

	return space.ResolveByName(ctx, clients.ClientFnBuilder(ctx, s.client), mg)
}
