package orgquota

import (
	"context"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	apisv1beta1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/v1beta1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/orgquota"
)

const (
	errTrackPCUsage      = "cannot track ProviderConfig usage"
	errTrackUsage        = "cannot track usage"
	errGetProviderConfig = "cannot get ProviderConfig or resolve credential references"
	errGetCreds          = "cannot get credentials"
	errNewClient         = "cannot create new client"
	errNotOrgQuota       = "managed resource is not a cloudfoundry OrgQuota"
	errGet               = "cannot get cloudfoundry OrgQuota"
	errCreate            = "cannot create cloudfoundry OrgQuota"
	errUpdate            = "cannot update cloudfoundry OrgQuota"
	errDelete            = "cannot delete cloudfoundry OrgQuota"
	errIDNotSet          = ".Status.AtProvider.ID is not set"
)

// externalConnecter specifies how the Reconciler should connect to
// the API used to sync and delete external resources.
type externalConnecter struct {
	kubeClient   k8s.Client
	usageTracker *resource.ProviderConfigUsageTracker
}

// externalConnecter type implements managed.ExternalConnecter
var _ managed.ExternalConnecter = &externalConnecter{}

// Connect method connects to the provider specified by the supplied
// managed resource and produce an ExternalClient.
func (c *externalConnecter) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	if _, ok := mg.(*v1alpha1.OrgQuota); !ok {
		return nil, errors.New(errNotOrgQuota)
	}

	if err := c.usageTracker.Track(ctx, mg.(resource.ModernManaged)); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	cf, err := clients.ClientFnBuilder(ctx, c.kubeClient)(mg)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &externalClient{cloudFoundryClient: orgquota.NewClient(cf), kubeClient: c.kubeClient}, nil
}

// Setup function builds a new controller that will be started by the
// provided Manager.
func Setup(mgr ctrl.Manager, controllerOptions controller.Options) error {
	name := managed.ControllerName(v1alpha1.OrgQuota_GroupKind)

	options := []managed.ReconcilerOption{
		managed.WithExternalConnecter(&externalConnecter{
			kubeClient:   mgr.GetClient(),
			usageTracker: resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1beta1.ProviderConfigUsage{}),
		}),
		managed.WithLogger(controllerOptions.Logger.WithValues("controller", name)),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithPollInterval(controllerOptions.PollInterval),
	}


	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.OrgQuota_GroupVersionKind),
		options...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(controllerOptions.ForControllerRuntime()).
		For(&v1alpha1.OrgQuota{}).
		Complete(ratelimiter.NewReconciler(name, r, controllerOptions.GlobalRateLimiter))
}

// Disconnect implements the managed.ExternalClient interface
func (c *externalClient) Disconnect(ctx context.Context) error {
	// No cleanup needed for Cloud Foundry client
	return nil
}

// externalClient manages the lifecycle of an external
// OrganizationQuota resource.
type externalClient struct {
	kubeClient         k8s.Client
	cloudFoundryClient orgquota.OrgQuota
}

// externalClient type implements the managed.ExternalClient interface
var _ managed.ExternalClient = &externalClient{}

// Observe the external resource the supplied Managed resource
// represents, if any.
func (e *externalClient) Observe(ctx context.Context, res resource.Managed) (managed.ExternalObservation, error) {
	managedOrgQuota, ok := res.(*v1alpha1.OrgQuota)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotOrgQuota)
	}

	external_name := meta.GetExternalName(managedOrgQuota)
	// If external name is not set, use metadata.name as default
	if external_name == "" {
		external_name = managedOrgQuota.GetName()
	}

	// get by external name
	externalOrgQuota, err := e.cloudFoundryClient.Get(ctx, external_name)

	// not found or error
	if err != nil {
		if clients.ErrorIsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errGet)
	}

	managedOrgQuota.SetConditions(xpv1.Available())
	lateInitialized := orgquota.LateInitialize(&managedOrgQuota.Spec.ForProvider, externalOrgQuota)
	managedOrgQuota.Status.AtProvider = orgquota.GenerateObservation(externalOrgQuota)

	if err := e.kubeClient.Status().Update(ctx, managedOrgQuota); err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errUpdate)
	}

	return managed.ExternalObservation{
		ResourceExists:          true,
		ResourceLateInitialized: lateInitialized,
		ResourceUpToDate:        !orgquota.NeedsReconciliation(managedOrgQuota),
	}, nil
}

// Create an external resource per the specifications of the supplied
// Managed resource. Called when Observe reports that the associated
// external resource does not exist.
func (e *externalClient) Create(ctx context.Context, res resource.Managed) (managed.ExternalCreation, error) {
	managedOrgQuota, ok := res.(*v1alpha1.OrgQuota)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotOrgQuota)
	}

	managedOrgQuota.SetConditions(xpv1.Creating())

	externalOrgQuota, err := e.cloudFoundryClient.Create(ctx, orgquota.GenerateCreateOrUpdate(managedOrgQuota.Spec.ForProvider))
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreate)
	}

	meta.SetExternalName(managedOrgQuota, externalOrgQuota.GUID)

	if err := e.kubeClient.Update(ctx, managedOrgQuota); err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errUpdate)
	}

	return managed.ExternalCreation{}, nil
}

// Update the external resource represented by the supplied Managed
// resource, if necessary. Called unless Observe reports that the
// associated external resource is up to date.
func (e *externalClient) Update(ctx context.Context, res resource.Managed) (managed.ExternalUpdate, error) {
	managedOrgQuota, ok := res.(*v1alpha1.OrgQuota)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotOrgQuota)
	}

	// assert that ID is set
	if managedOrgQuota.Status.AtProvider.ID == nil {
		return managed.ExternalUpdate{}, errors.New(errUpdate)
	}

	_, err := e.cloudFoundryClient.Update(ctx, *managedOrgQuota.Status.AtProvider.ID, orgquota.GenerateCreateOrUpdate(managedOrgQuota.Spec.ForProvider))
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdate)
	}

	return managed.ExternalUpdate{}, nil
}

// Delete the external resource upon deletion of its associated Managed
// resource. Called when the managed resource has been deleted.
func (e *externalClient) Delete(ctx context.Context, res resource.Managed) (managed.ExternalDelete, error) {
	managedOrgQuota, ok := res.(*v1alpha1.OrgQuota)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotOrgQuota)
	}
	managedOrgQuota.SetConditions(xpv1.Deleting())

	// assert that ID is set
	if managedOrgQuota.Status.AtProvider.ID == nil {
		return managed.ExternalDelete{}, errors.Wrap(errors.New(errIDNotSet), errDelete)
	}

	_, err := e.cloudFoundryClient.Delete(ctx, *managedOrgQuota.Status.AtProvider.ID)
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, errDelete)
	}

	return managed.ExternalDelete{}, nil
}
