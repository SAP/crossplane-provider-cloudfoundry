package spacemembers

import (
	"context"

	"github.com/pkg/errors"

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

	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/apis/members/v1alpha1"
	apisv1alpha1 "github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/apis/v1alpha1"
	apisv1beta1 "github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/apis/v1beta1"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/clients"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/clients/cfclient"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/features"
)

const (
	errTrackPCUsage     = "cannot track ProviderConfig usage"
	errGetPC            = "cannot get ProviderConfig"
	errGetCreds         = "cannot get credentials"
	errNewClient        = "cannot create new client"
	errNotSpaceMembers  = "managed resource is not a cloudfoundry SpaceMembers"
	errRead             = "cannot read cloudfoundry SpaceMembers"
	errCreate           = "cannot create cloudfoundry SpaceMembers"
	errUpdate           = "cannot update cloudfoundry SpaceMembers"
	errDelete           = "cannot delete cloudfoundry SpaceMembers"
	errSpaceNotResolved = "Space reference is not resolved."
)

// Setup adds a controller that reconciles managed resources SpaceMembers.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.SpaceMembersGroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
	if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), apisv1alpha1.StoreConfigGroupVersionKind))
	}

	options := []managed.ReconcilerOption{

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
		resource.ManagedKind(v1alpha1.SpaceMembersGroupVersionKind),
		options...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&v1alpha1.SpaceMembers{}).
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
	if _, ok := mg.(*v1alpha1.SpaceMembers); !ok {
		return nil, errors.New(errNotSpaceMembers)
	}

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	client, err := c.newClientFn(ctx, c.kube, mg)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &external{client: client}, nil
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	// A 'client' used to connect to the external resource API, in this case the Cloud Foundry v3 API.
	client *cfclient.Client
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.SpaceMembers)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotSpaceMembers)
	}

	// Check that reference to Space is resolved
	if cr.Spec.ForProvider.Space == nil {
		return managed.ExternalObservation{}, errors.New(errSpaceNotResolved)
	}

	if meta.GetExternalName(cr) == "" || meta.GetExternalName(cr) != *cr.Spec.ForProvider.Space {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	observed, err := c.client.ObserveSpaceMembers(ctx, cr)

	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errRead)
	}

	if observed == nil {
		return managed.ExternalObservation{
			ResourceExists:   cr.Status.AtProvider.AssignedRoles != nil,
			ResourceUpToDate: false,
		}, nil
	}

	// Set external names
	cr.Status.AtProvider.AssignedRoles = observed.AssignedRoles
	cr.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: true,
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.SpaceMembers)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotSpaceMembers)
	}

	// TODO: checking conflicting CR that `strictly` enforces the same role on the same
	cr.SetConditions(xpv1.Creating())

	created, err := c.client.AssignSpaceMembers(ctx, cr)

	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreate)
	}

	// Set external names
	meta.SetExternalName(cr, *cr.Spec.ForProvider.Space)

	// Directly set observation instead of external names, as the collection does not have a single identity.
	cr.Status.AtProvider.AssignedRoles = created.AssignedRoles

	return managed.ExternalCreation{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.SpaceMembers)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotSpaceMembers)
	}

	updated, err := c.client.UpdateSpaceMembers(ctx, cr)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdate)
	}

	cr.Status.AtProvider.AssignedRoles = updated.AssignedRoles

	return managed.ExternalUpdate{
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.SpaceMembers)
	if !ok {
		return errors.New(errNotSpaceMembers)
	}

	cr.SetConditions(xpv1.Deleting())

	// nothing to delete
	if cr.Status.AtProvider.AssignedRoles == nil || len(cr.Status.AtProvider.AssignedRoles) == 0 {
		return nil
	}

	err := c.client.DeleteSpaceMembers(ctx, cr)
	if err != nil {
		return errors.Wrap(err, errDelete)
	}

	return nil
}
