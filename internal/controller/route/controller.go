package route

import (
	"context"

	"github.com/pkg/errors"

	cf "github.com/cloudfoundry/go-cfclient/v3/client"

	ctrl "sigs.k8s.io/controller-runtime"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/connection"
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha2"
	apisv1alpha1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/v1alpha1"
	apisv1beta1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/v1beta1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/route"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/features"
)

type RouteService interface {
	GetByIDOrSpec(ctx context.Context, guid string, forProvider v1alpha2.RouteParameters) (*v1alpha2.RouteObservation, error)
	Create(ctx context.Context, forProvider v1alpha2.RouteParameters) (string, error)
	Update(ctx context.Context, guid string, forProvider v1alpha2.RouteParameters) error
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
	name := managed.ControllerName(v1alpha2.RouteGroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
	if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), apisv1alpha1.StoreConfigGroupVersionKind))
	}

	options := []managed.ReconcilerOption{
		managed.WithInitializers(),
		managed.WithExternalConnecter(&connector{
			kube:        mgr.GetClient(),
			usage:       resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1beta1.ProviderConfigUsage{}),
			newClientFn: clients.CloudfoundryClientBuilder}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithConnectionPublishers(cps...),
		managed.WithPollInterval(o.PollInterval),
	}

	if o.Features.Enabled(features.EnableBetaManagementPolicies) {
		options = append(options, managed.WithManagementPolicies())
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha2.RouteGroupVersionKind),
		options...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&v1alpha2.Route{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector is expected to produce an ExternalClient when its Connect method
// is called.
type connector struct {
	kube        k8s.Client
	usage       resource.Tracker
	newClientFn func(context.Context, k8s.Client, resource.Managed) (*cf.Client, error)
}

// Connect typically produces an ExternalClient by:
// 1. Tracking that the managed resource is using a ProviderConfig.
// 2. Getting the managed resource's ProviderConfig.
// 3. Getting the credentials specified by the ProviderConfig.
// 4. Using the credentials to form a client.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	if _, ok := mg.(*v1alpha2.Route); !ok {
		return nil, errors.New(errNotRoute)
	}

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	cfv3, err := c.newClientFn(ctx, c.kube, mg)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &external{RouteService: route.NewClient(cfv3), kube: c.kube}, nil
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
	cr, ok := mg.(*v1alpha2.Route)
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
	cr, ok := mg.(*v1alpha2.Route)
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
	cr, ok := mg.(*v1alpha2.Route)
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
func (c *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha2.Route)
	if !ok {
		return errors.New(errNotRoute)
	}

	// Prevent delete if there are bindings.
	if len(cr.Status.AtProvider.Destinations) > 0 {
		return errors.New(errActiveBinding)
	}

	cr.SetConditions(xpv1.Deleting())

	return c.RouteService.Delete(ctx, meta.GetExternalName(cr))

}
