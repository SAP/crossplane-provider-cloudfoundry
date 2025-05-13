package spacerole

import (
	"context"

	"github.com/pkg/errors"

	"k8s.io/utils/ptr"
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

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	scv1alpha1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/v1alpha1"
	pcv1beta1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/v1beta1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/job"
	role "github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/role"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/space"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/features"
)

const (
	errWrongKind         = "Managed resource is not an SpaceRole kind"
	errTrackUsage        = "cannot track usage"
	errGetProviderConfig = "cannot get ProviderConfig or resolve credential references"
	errGetClient         = "cannot create a client to talk to the cloudfoundry API"
	errGet               = "cannot get space role according to the specified parameters"
	errGetResource       = "cannot get space role via the cloudfoundry API"
	errCreate            = "cannot create space role"
	errDelete            = "cannot delete space role"
)

// Setup adds a controller that reconciles SpaceRole resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.SpaceRole_GroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
	if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), scv1alpha1.StoreConfigGroupVersionKind))
	}

	options := []managed.ReconcilerOption{
		managed.WithExternalConnecter(&connector{kube: mgr.GetClient(),
			usage: resource.NewProviderConfigUsageTracker(mgr.GetClient(), &pcv1beta1.ProviderConfigUsage{}),
		}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithConnectionPublishers(cps...),
		managed.WithInitializers(&initializer{
			kube: mgr.GetClient(),
		}),
	}

	if o.Features.Enabled(features.EnableBetaManagementPolicies) {
		options = append(options, managed.WithManagementPolicies())
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.SpaceRole_GroupVersionKind),
		options...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&v1alpha1.SpaceRole{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector supplies a function for the Reconciler to create a client to the external CloudFoundry resources.
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
	if _, ok := mg.(*v1alpha1.SpaceRole); !ok {
		return nil, errors.New(errWrongKind)
	}

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackUsage)
	}

	cf, err := clients.ClientFnBuilder(ctx, c.kube)(mg)
	if err != nil {
		return nil, errors.Wrap(err, errGetClient)
	}

	role, job := role.NewClient(cf)
	return &external{role: role, kube: c.kube, job: job}, nil
}

// An external is a managed.ExternalConnecter that is using the CloudFoundry API to observe and modify resources.
type external struct {
	role role.Role
	job  job.Job
	kube k8s.Client
}

// Observe managed resource SpaceRole
func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.SpaceRole)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errWrongKind)
	}

	// Fetch the role object using the CloudFoundry API by guid or according to the specified parameters
	guid := meta.GetExternalName(cr)
	r, err := role.GetSpaceRole(ctx, c.role, guid, cr.Spec.ForProvider)

	if err != nil {
		if clients.ErrorIsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errGet)
	}

	if r == nil {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	resourceLateInitialized := false
	if guid != r.GUID {
		meta.SetExternalName(cr, r.GUID)
		resourceLateInitialized = true
	}

	cr.Status.AtProvider = role.GenerateSpaceRoleObservation(r)
	cr.Status.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:          cr.Status.AtProvider.ID != nil,
		ResourceUpToDate:        true,
		ResourceLateInitialized: resourceLateInitialized,
	}, nil
}

// Create a managed resource SpaceRole
func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.SpaceRole)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errWrongKind)
	}

	spec := cr.Spec.ForProvider
	if spec.Space == nil || spec.Username == "" || spec.Type == "" {
		return managed.ExternalCreation{}, errors.New(errCreate)
	}

	o, err := c.role.CreateSpaceRoleWithUsername(ctx, *spec.Space, spec.Username, role.SpaceRoleType(spec.Type), ptr.Deref(spec.Origin, "sap.ids"))
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

// Update managed resource SpaceRole
func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	_, ok := mg.(*v1alpha1.SpaceRole)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errWrongKind)
	}

	// Do nothing, as SpaceRole is observe-only

	return managed.ExternalUpdate{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

// Delete managed resource SpaceRole
func (c *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.SpaceRole)
	if !ok {
		return errors.New(errWrongKind)
	}
	// TODO

	cr.SetConditions(xpv1.Deleting())
	if cr.Status.AtProvider.ID == nil {
		return nil
	}

	// Delete is async and we need to implement wait for deletion
	jobGUID, err := c.role.Delete(ctx, *cr.Status.AtProvider.ID)
	if err != nil {
		return errors.Wrap(err, errDelete)
	}

	return job.PollJobComplete(ctx, c.job, jobGUID)
}

type initializer struct {
	kube k8s.Client
}

// / Initialize implements the Initializer interface
func (c *initializer) Initialize(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.SpaceRole)
	if !ok {
		return errors.New(errWrongKind)
	}

	if cr.Spec.ForProvider.SpaceRef != nil || cr.Spec.ForProvider.SpaceSelector != nil {
		return cr.ResolveReferences(ctx, c.kube)
	}

	return space.ResolveByName(ctx, clients.ClientFnBuilder(ctx, c.kube), mg)
}
