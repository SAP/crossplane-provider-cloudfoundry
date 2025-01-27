package space

import (
	"context"

	cf "github.com/cloudfoundry-community/go-cfclient/v3/client"
	cfresource "github.com/cloudfoundry-community/go-cfclient/v3/resource"
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

	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/apis/space/v1alpha1"
	apisv1alpha1 "github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/apis/v1alpha1"
	apisv1beta1 "github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/apis/v1beta1"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/clients"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/clients/cfclient"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/features"
)

const (
	errTrackPCUsage = "cannot track ProviderConfig usage"
	errGetPC        = "cannot get ProviderConfig"
	errGetCreds     = "cannot get credentials"
	errNewClient    = "cannot create new client"
	errNotSpace     = "managed resource is not a cloudfoundry Space"
	errGet          = "cannot get cloudfoundry Space"
	errCreate       = "cannot create cloudfoundry Space"
	errUpdate       = "cannot update cloudfoundry Space"
	errDelete       = "cannot delete cloudfoundry Space"
)

// Setup adds a controller that reconciles Org managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.Space_GroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
	if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), apisv1alpha1.StoreConfigGroupVersionKind))
	}

	options := []managed.ReconcilerOption{

		managed.WithExternalConnecter(&connector{
			kube:        mgr.GetClient(),
			usage:       resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1beta1.ProviderConfigUsage{}),
			newClientFn: clients.CloudfoundryClientBuilder,
		}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithConnectionPublishers(cps...),
		managed.WithPollInterval(o.PollInterval),
	}

	if o.Features.Enabled(features.EnableBetaManagementPolicies) {
		options = append(options,
			managed.WithManagementPolicies())
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.Space_GroupVersionKind),
		options...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&v1alpha1.Space{}).
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
	if _, ok := mg.(*v1alpha1.Space); !ok {
		return nil, errors.New(errNotSpace)
	}

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	s, err := c.newClientFn(ctx, c.kube, mg)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &external{service: s.Spaces}, nil
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	// A 'client' used to connect to the external resource API. In practice this
	// would be something like an AWS SDK client.
	service *cf.SpaceClient
}

// Observe generates observation for a space
func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.Space)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotSpace)
	}

	if meta.GetExternalName(cr) == "" {
		return managed.ExternalObservation{}, nil
	}

	s, err := c.service.Single(ctx,
		&cf.SpaceListOptions{
			ListOptions:       nil,
			Names:             cf.Filter{Values: []string{*cr.Spec.ForProvider.Name}},
			OrganizationGUIDs: cf.Filter{Values: []string{*cr.Spec.ForProvider.Org}},
		},
	)
	if err != nil {
		if IsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{ResourceExists: false}, errors.Wrap(err, errGet)
	}

	cr.Status.AtProvider = GenerateObservation(s)

	if cr.Status.AtProvider.ID != nil {
		cr.SetConditions(xpv1.Available())
	}

	return managed.ExternalObservation{
		ResourceExists:   cr.Status.AtProvider.ID != nil,
		ResourceUpToDate: s.Name == *cr.Spec.ForProvider.Name,
	}, nil
}

// Create creates a space
func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.Space)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotSpace)
	}

	cr.SetConditions(xpv1.Creating())
	fp := cr.Spec.ForProvider
	s, err := c.service.Create(ctx, cfresource.NewSpaceCreate(*fp.Name, *fp.Org))
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreate)
	}

	meta.SetExternalName(cr, s.GUID)

	return managed.ExternalCreation{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

// Update updates a space
func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.Space)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotSpace)
	}

	if cr.Status.AtProvider.ID == nil {
		return managed.ExternalUpdate{}, errors.New(errUpdate)
	}
	_, err := c.service.Update(ctx, *cr.Status.AtProvider.ID, &cfresource.SpaceUpdate{
		Name:     *cr.Spec.ForProvider.Name,
		Metadata: &cfresource.Metadata{},
	})
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdate)
	}

	return managed.ExternalUpdate{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

// Delete depetes a space
func (c *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.Space)
	if !ok {
		return errors.New(errNotSpace)
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

// GenerateObservation generates observations for Spaces
func GenerateObservation(s *cfresource.Space) v1alpha1.SpaceObservation {
	o := v1alpha1.SpaceObservation{}
	if s == nil {
		return o
	}
	o.ID = &s.GUID
	return o
}

// IsNotFound checks if an error is a not found error from CF
func IsNotFound(err error) bool {
	return err.Error() == cf.ErrExactlyOneResultNotReturned.Error()
}
