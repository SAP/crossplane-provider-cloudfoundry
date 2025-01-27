package route

import (
	"context"
	"reflect"
	"strings"

	"github.com/pkg/errors"

	cf "github.com/cloudfoundry-community/go-cfclient/v3/client"
	cfresource "github.com/cloudfoundry-community/go-cfclient/v3/resource"

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

	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/apis/route/v1alpha1"
	apisv1alpha1 "github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/apis/v1alpha1"
	apisv1beta1 "github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/apis/v1beta1"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/clients"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/clients/cfclient"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/features"
)

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
	kube        k8s.Client
	usage       resource.Tracker
	newClientFn func(context.Context, k8s.Client, resource.Managed) (*cfclient.Client, error)
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

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	s, err := c.newClientFn(ctx, c.kube, mg)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &external{service: s.Routes, client: s, kube: c.kube}, nil
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	// A 'client' used to connect to the external resource API. In practice this
	// would be something like an AWS SDK client.
	kube    k8s.Client
	client  *cfclient.Client
	service *cf.RouteClient
}

// Observe generates observation for Route's
func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.Route)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotRoute)
	}

	if meta.GetExternalName(cr) == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	r, err := c.service.Get(ctx, meta.GetExternalName(cr))
	if err != nil {
		if cfErrorIsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errGet)
	}

	if err != nil || r.GUID == "" {
		return managed.ExternalObservation{ResourceExists: false}, err
	}

	current := cr.Spec.ForProvider.DeepCopy()
	lateInitialize(&cr.Spec.ForProvider, r)
	if !reflect.DeepEqual(current, &cr.Spec.ForProvider) {
		if err := c.kube.Update(ctx, cr); err != nil {
			return managed.ExternalObservation{}, errors.Wrap(err, errUpdate)
		}
	}

	cr.Status.AtProvider = generateObservation(r)

	if cr.Status.AtProvider.ID != nil {
		cr.SetConditions(xpv1.Available())
	}

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: true,
	}, nil

}

// Create a route
func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.Route)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotRoute)
	}

	cr.SetConditions(xpv1.Creating())
	fp := cr.Spec.ForProvider

	if fp.Domain.ID != nil {
		_, err := c.client.Domains.Get(ctx, *fp.Domain.ID)
		if err != nil {
			return managed.ExternalCreation{}, errors.Wrap(err, "cannot get specified by guid")
		}
	}

	if fp.Domain.ID == nil {
		if fp.Domain.Name == nil {
			return managed.ExternalCreation{}, errors.New("Domain or DomainName must be provided")
		}

		d, err := c.client.Domains.Single(ctx,
			&cf.DomainListOptions{
				Names: cf.Filter{Values: []string{*fp.Domain.Name}},
			})
		if err != nil {
			return managed.ExternalCreation{}, errors.Wrap(err, "cannot get domain by name")
		}
		fp.Domain.ID = &d.GUID
	}

	rc := cfresource.NewRouteCreate(*fp.Domain.ID, *fp.Space)
	rc.Host = fp.Hostname
	rc.Path = fp.Path
	if fp.Port != nil {
		rc.Port = fp.Port
	}

	r, err := c.service.Create(ctx, rc)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreate)
	}

	meta.SetExternalName(cr, r.GUID)

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

	if cr.Status.AtProvider.ID == nil {
		return managed.ExternalUpdate{}, errors.New(errUpdate)
	}

	// TODO

	return managed.ExternalUpdate{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

// Delete deletes a route
func (c *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.Route)
	if !ok {
		return errors.New(errNotRoute)
	}

	// prevent delete if there are bindings.
	if len(cr.Spec.ForProvider.Destinations) > 0 {
		return errors.New(errActiveBinding)
	}

	cr.SetConditions(xpv1.Deleting())

	if cr.Status.AtProvider.ID == nil {
		return errors.New(errDelete)
	}
	_, err := c.service.Delete(ctx, *cr.Status.AtProvider.ID)
	if err != nil {
		return errors.Wrap(err, errDelete)
	}
	return nil
}

func lateInitialize(spec *v1alpha1.RouteParameters, route *cfresource.Route) {
	spec.Destinations = nil
	if route.Destinations != nil {
		for _, d := range route.Destinations {
			dest := v1alpha1.DestinationParameters{App: d.App.GUID, Port: d.Port}

			spec.Destinations = append(spec.Destinations, dest)
		}

	}
}

func generateObservation(r *cfresource.Route) v1alpha1.RouteObservation {
	o := v1alpha1.RouteObservation{}
	if r == nil {
		return o
	}
	o.ID = &r.GUID
	o.Endpoint = &r.URL
	return o
}

// DomainFinder contains helper to retrieve domains from cf api
type DomainFinder struct {
	client cfclient.Client
	kube   k8s.Client
}

// Initialize helps to retrieve domain id based on domain name
func (d *DomainFinder) Initialize(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.Route)
	if !ok {
		return errors.New(errNotRoute)
	}
	fp := cr.Spec.ForProvider

	if fp.Domain.ID != nil || fp.Domain.Name == nil {
		return nil
	}

	domain, err := d.client.Domains.Single(ctx,
		&cf.DomainListOptions{
			Names: cf.Filter{Values: []string{*fp.Domain.Name}},
		})
	if err != nil {
		return errors.Wrap(err, "error getting domain by name")
	}

	cr.Spec.ForProvider.Domain.ID = &domain.GUID

	return d.kube.Update(ctx, cr)
}

func cfErrorIsNotFound(err error) bool {
	// nolint: errorlint
	return strings.Contains(err.Error(), "NotFound")
}
