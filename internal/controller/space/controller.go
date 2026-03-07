package space

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
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/org"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/space"
)

const (
	errTrackPCUsage      = "cannot track ProviderConfig usage"
	errTrackUsage        = "cannot track usage"
	errGetProviderConfig = "cannot get ProviderConfig or resolve credential references"
	errGetCreds          = "cannot get credentials"
	errNewClient         = "cannot create new client"
	errNotSpace          = "managed resource is not a cloudfoundry Space"
	errGet               = "cannot get cloudfoundry Space"
	errCreate            = "cannot create cloudfoundry Space"
	errUpdate            = "cannot update cloudfoundry Space"
	errDelete            = "cannot delete cloudfoundry Space"
	errEnableSSH         = "cannot enable SSH for space"
)

// Setup adds a controller that reconciles Org managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.Space_GroupKind)

	options := []managed.ReconcilerOption{

		managed.WithExternalConnecter(&connector{
			kube:  mgr.GetClient(),
			usage: resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1beta1.ProviderConfigUsage{}),
		}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithPollInterval(o.PollInterval),
		managed.WithInitializers(&orgInitializer{
			kube: mgr.GetClient(),
		}),
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
	kube  k8s.Client
	usage *resource.ProviderConfigUsageTracker
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

	if err := c.usage.Track(ctx, mg.(resource.ModernManaged)); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	cf, err := clients.ClientFnBuilder(ctx, c.kube)(mg)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	spaceClient, featureClient, _ := space.NewClient(cf)

	return &external{
		kube:    c.kube,
		client:  spaceClient,
		feature: featureClient,
	}, nil

}

// Disconnect implements the managed.ExternalClient interface
func (c *external) Disconnect(ctx context.Context) error {
	// No cleanup needed for Cloud Foundry client
	return nil
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	kube    k8s.Client
	client  space.Space
	feature space.Feature
}

// Observe generates observation for a space
func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.Space)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotSpace)
	}

	// Check if the external resource exists
	guid := meta.GetExternalName(cr)

	s, err := space.GetByIDOrSpec(ctx, c.client, guid, cr.Spec.ForProvider)

	// not found or error
	if err != nil {
		if clients.ErrorIsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{ResourceExists: false}, errors.Wrap(err, errGet)
	}

	if s == nil {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	// ssh
	ssh, err := c.feature.IsSSHEnabled(ctx, s.GUID)
	if err != nil {
		// new error message cannot get space feature
		return managed.ExternalObservation{}, errors.Wrap(err, errGet)
	}

	resourceLateInitialized := space.LateInitialize(cr, s, ssh)
	// update external name, if needed
	if guid != s.GUID {
		meta.SetExternalName(cr, s.GUID)
		resourceLateInitialized = true // force update
	}

	cr.Status.AtProvider = space.GenerateObservation(s, ssh)
	cr.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:          true,
		ResourceUpToDate:        space.IsUpToDate(cr.Spec.ForProvider, s, ssh),
		ResourceLateInitialized: resourceLateInitialized,
	}, nil
}

// Create creates a space
func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.Space)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotSpace)
	}

	cr.SetConditions(xpv1.Creating())

	s, err := c.client.Create(ctx, space.GenerateCreate(cr.Spec.ForProvider))
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreate)
	}

	meta.SetExternalName(cr, s.GUID)

	if err := c.kube.Update(ctx, cr); err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errUpdate)
	}

	// enable SSH if allowed
	if cr.Spec.ForProvider.AllowSSH {
		err = c.feature.EnableSSH(ctx, s.GUID, true)
		if err != nil {
			return managed.ExternalCreation{}, errors.Wrap(err, errEnableSSH)
		}
	}

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

	// assert that ID is set
	if !clients.IsValidGUID(cr.Status.AtProvider.ID) {
		return managed.ExternalUpdate{}, errors.New(errUpdate)
	}

	// reconcile SSH
	if cr.Spec.ForProvider.AllowSSH != cr.Status.AtProvider.AllowSSH {
		err := c.feature.EnableSSH(ctx, cr.Status.AtProvider.ID, cr.Spec.ForProvider.AllowSSH)
		if err != nil {
			return managed.ExternalUpdate{}, errors.Wrap(err, errEnableSSH)
		}
	}

	// rename
	if cr.Spec.ForProvider.Name != cr.Status.AtProvider.Name {
		_, err := c.client.Update(ctx, cr.Status.AtProvider.ID, space.GenerateUpdate(cr.Spec.ForProvider))
		if err != nil {
			return managed.ExternalUpdate{}, errors.Wrap(err, errUpdate)
		}
	}

	return managed.ExternalUpdate{}, nil
}

// Delete deletes a space
func (c *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1alpha1.Space)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotSpace)
	}
	cr.SetConditions(xpv1.Deleting())

	// assert that ID is set
	if !clients.IsValidGUID(cr.Status.AtProvider.ID) {
		return managed.ExternalDelete{}, errors.New(errDelete)
	}

	_, err := c.client.Delete(ctx, cr.Status.AtProvider.ID)
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, errDelete)
	}

	return managed.ExternalDelete{}, nil
}

type initializer struct {
	kube k8s.Client
}

type orgInitializer initializer

// / Initialize implements the Initializer interface
func (c *orgInitializer) Initialize(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.Space)
	if !ok {
		return errors.New(errNotSpace)
	}

	if cr.Spec.ForProvider.OrgRef != nil || cr.Spec.ForProvider.OrgSelector != nil {
		return cr.ResolveReferences(ctx, c.kube)
	}

	// If orgName is provided, resolve by orgName
	if cr.Spec.ForProvider.OrgName != nil {
		return org.ResolveByName(ctx, clients.ClientFnBuilder(ctx, c.kube), mg)
	}

	return nil
}
