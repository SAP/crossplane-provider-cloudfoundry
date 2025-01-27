package orgmembers

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
	errTrackPCUsage   = "cannot track ProviderConfig usage"
	errGetPC          = "cannot get ProviderConfig"
	errGetCreds       = "cannot get credentials"
	errNewClient      = "cannot create new client"
	errNotOrgMembers  = "managed resource is not a cloudfoundry OrgMembers"
	errRead           = "cannot read cloudfoundry OrgMembers"
	errCreate         = "cannot create cloudfoundry OrgMembers"
	errUpdate         = "cannot update cloudfoundry OrgMembers"
	errDelete         = "cannot delete cloudfoundry OrgMembers"
	errOrgNotResolved = "org reference is not resolved."
)

// Setup adds a controller that reconciles managed resources OrgMembers.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.OrgMembersGroupKind)

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
	usage       resource.Tracker
	newClientFn func(context.Context, k8s.Client, resource.Managed) (*cfclient.Client, error)
}

// Connect typically produces an ExternalClient by:
// 1. Tracking that the managed resource is using a ProviderConfig.
// 2. Getting the managed resource's ProviderConfig.
// 3. Getting the credentials specified by the ProviderConfig.
// 4. Using the credentials to form a client.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	if _, ok := mg.(*v1alpha1.OrgMembers); !ok {
		return nil, errors.New(errNotOrgMembers)
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
	cr, ok := mg.(*v1alpha1.OrgMembers)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotOrgMembers)
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
		return managed.ExternalCreation{}, errors.New(errNotOrgMembers)
	}

	// TODO: checking conflicting CR that `strictly` enforces the same role on the same
	cr.SetConditions(xpv1.Creating())

	created, err := c.client.AssignOrgMembers(ctx, cr)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreate)
	}

	// Set external names
	meta.SetExternalName(cr, string(cr.Spec.ForProvider.RoleType)+"@"+*cr.Spec.ForProvider.Org)

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
		return managed.ExternalUpdate{}, errors.New(errNotOrgMembers)
	}

	updated, err := c.client.UpdateOrgMembers(ctx, cr)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdate)
	}

	// Update external names
	meta.SetExternalName(cr, string(cr.Spec.ForProvider.RoleType)+"@"+*cr.Spec.ForProvider.Org)

	// Directly set observation to the updated
	cr.Status.AtProvider.AssignedRoles = updated.AssignedRoles

	return managed.ExternalUpdate{
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.OrgMembers)
	if !ok {
		return errors.New(errNotOrgMembers)
	}
	cr.SetConditions(xpv1.Deleting())

	// TODO: make sure there is at least one manager of the org?
	// TODO: In case of deletion error for some roles, this resource will stuck in a false status (READY=false and SYNCED=false). We need a strategy to handle this.
	// 		 e.g., organization_user role cannot be deleted if the user has role in some spaces in the same org.
	err := c.client.DeleteOrgMembers(ctx, cr)
	if err != nil {
		return errors.Wrap(err, errDelete)
	}

	// clear members
	cr.Status.AtProvider.AssignedRoles = nil
	return nil
}
