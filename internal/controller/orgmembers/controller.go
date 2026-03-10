package orgmembers

import (
	"context"

	"github.com/cloudfoundry/go-cfclient/v3/config"
	"github.com/pkg/errors"

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
	apisv1beta1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/v1beta1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/members"
)

const (
	errWrongKind         = "Managed resource is not an OrgMembers kind"
	errTrackUsage        = "cannot track usage"
	errGetProviderConfig = "cannot get ProviderConfig or resolve credential references"
	errGetClient         = "cannot create a client to talk to the cloudfoundry API"
	errGetCreds          = "cannot get credentials"
	errRead              = "cannot read cloudfoundry OrgMembers"
	errCreate            = "cannot create cloudfoundry OrgMembers"
	errUpdate            = "cannot update cloudfoundry OrgMembers"
	errDelete            = "cannot delete cloudfoundry OrgMembers"
	errOrgNotResolved    = "org reference is not resolved."
)

// Setup adds a controller that reconciles managed resources OrgMembers.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.OrgMembersGroupKind)

	options := []managed.ReconcilerOption{
		managed.WithExternalConnecter(&connector{
			kube:        mgr.GetClient(),
			usage:       resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1beta1.ProviderConfigUsage{}),
			newClientFn: members.NewClient}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithPollInterval(o.PollInterval),
	}


	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.OrgMembersGroupVersionKind),
		options...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&v1alpha1.OrgMembers{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector is expected to produce an ExternalClient when its Connect method
// is called.
type connector struct {
	kube        k8s.Client
	usage *resource.ProviderConfigUsageTracker
	newClientFn func(*config.Config) (*members.Client, error)
}

// Connect typically produces an ExternalClient by:
// 1. Tracking that the managed resource is using a ProviderConfig.
// 2. Getting the managed resource's ProviderConfig.
// 3. Getting the credentials specified by the ProviderConfig.
// 4. Using the credentials to form a client.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	if _, ok := mg.(*v1alpha1.OrgMembers); !ok {
		return nil, errors.New(errWrongKind)
	}

	if err := c.usage.Track(ctx, mg.(resource.ModernManaged)); err != nil {
		return nil, errors.Wrap(err, errTrackUsage)
	}

	cfg, err := clients.GetCredentialConfig(ctx, c.kube, mg)
	if err != nil {
		return nil, errors.Wrap(err, errGetProviderConfig)
	}

	client, err := c.newClientFn(cfg)
	if err != nil {
		return nil, errors.Wrap(err, errGetClient)
	}

	return &external{client: client}, nil
}

// Disconnect implements the managed.ExternalClient interface
func (c *external) Disconnect(ctx context.Context) error {
	// No cleanup needed for Cloud Foundry client
	return nil
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	// A 'client' used to connect to the external resource API, in this case the Cloud Foundry v3 API.
	client *members.Client
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.OrgMembers)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errWrongKind)
	}

	// Reference to Org must be resolved first
	if cr.Spec.ForProvider.Org == nil {
		return managed.ExternalObservation{}, errors.New(errOrgNotResolved)
	}

	// Observe external state and compile an observation if the states are consistent with the CR,
	// otherwise a nil observation is returned
	observed, err := c.client.ObserveOrgMembers(ctx, cr)

	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errRead)
	}

	// external state is not consistent with CR
	if observed == nil {
		return managed.ExternalObservation{
			ResourceExists:   cr.Status.AtProvider.AssignedRoles != nil,
			ResourceUpToDate: false,
		}, nil
	}

	cr.Status.AtProvider.AssignedRoles = observed.AssignedRoles
	cr.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: true,
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.OrgMembers)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errWrongKind)
	}

	// TODO: checking conflicting CR that `strictly` enforces the same role on the same
	cr.SetConditions(xpv1.Creating())

	created, err := c.client.AssignOrgMembers(ctx, cr)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreate)
	}

	// Set external names
	meta.SetExternalName(cr, cr.Spec.ForProvider.RoleType+"@"+*cr.Spec.ForProvider.Org)

	// Directly set observation instead of external names, as the collection does not have a single identity.
	cr.Status.AtProvider.AssignedRoles = created.AssignedRoles

	return managed.ExternalCreation{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.OrgMembers)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errWrongKind)
	}

	updated, err := c.client.UpdateOrgMembers(ctx, cr)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdate)
	}

	// Update external names
	meta.SetExternalName(cr, cr.Spec.ForProvider.RoleType+"@"+*cr.Spec.ForProvider.Org)

	// Directly set observation to the updated
	cr.Status.AtProvider.AssignedRoles = updated.AssignedRoles

	return managed.ExternalUpdate{
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1alpha1.OrgMembers)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errWrongKind)
	}
	cr.SetConditions(xpv1.Deleting())

	// TODO: make sure there is at least one manager of the org?
	// TODO: In case of deletion error for some roles, this resource will stuck in a false status (READY=false and SYNCED=false). We need a strategy to handle this.
	// 		 e.g., organization_user role cannot be deleted if the user has role in some spaces in the same org.
	err := c.client.DeleteOrgMembers(ctx, cr)
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, errDelete)
	}

	// clear members
	cr.Status.AtProvider.AssignedRoles = nil
	return managed.ExternalDelete{}, nil
}
