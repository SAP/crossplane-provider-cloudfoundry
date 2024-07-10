package servicekey

import (
	"context"

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

	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/apis/servicekey/v1alpha1"
	apisv1alpha1 "github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/apis/v1alpha1"
	apisv1beta1 "github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/apis/v1beta1"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/clients"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/clients/cfclient"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/clients/servicekey"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/features"
)

const (
	resourceType    = "ServiceKey"
	externalSystem  = "Cloud Foundry"
	errTrackPCUsage = "cannot track ProviderConfig usage"
	errNewClient    = "cannot create a client for " + externalSystem
	errWrongCRType  = "managed resource is not a " + resourceType
	errGet          = "cannot get " + resourceType + " in " + externalSystem
	errCreate       = "cannot create " + resourceType + " in " + externalSystem
	errDelete       = "cannot delete " + resourceType + " in " + externalSystem
)

// Setup adds a controller that reconciles ServiceKey CR.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.ServiceKeyGroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
	if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), apisv1alpha1.StoreConfigGroupVersionKind))
	}

	options := []managed.ReconcilerOption{
		managed.WithInitializers(),
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
		options = append(options, managed.WithManagementPolicies())
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.ServiceKeyGroupVersionKind),
		options...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&v1alpha1.ServiceKey{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector is expected to produce an external client when its Connect method
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
	if _, ok := mg.(*v1alpha1.ServiceKey); !ok {
		return nil, errors.New(errWrongCRType)
	}

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	cf, err := c.newClientFn(ctx, c.kube, mg)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &external{
		kube:       c.kube,
		servicekey: servicekey.NewClient(cf),
	}, nil
}

// An external service observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	kube       k8s.Client
	servicekey *servicekey.Client
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.ServiceKey)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errWrongCRType)
	}

	// Try to get external resource by external_name
	r, err := c.servicekey.MatchSingle(ctx, cr.Spec.ForProvider)

	if err != nil {
		if cfclient.ErrorIsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errGet)
	}

	// Observed
	if meta.GetExternalName(cr) == "" {
		meta.SetExternalName(cr, r.GUID)
	}
	servicekey.LateInitialize(&cr.Spec.ForProvider, r)
	cr.Status.AtProvider = servicekey.GenerateObservation(r)
	cr.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:    true,
		ResourceUpToDate:  servicekey.IsUpToDate(&cr.Spec.ForProvider, r),
		ConnectionDetails: c.servicekey.GetConnectionDetails(ctx, r.GUID, cr.Spec.ForProvider.ConnectionDetailsAsJSON),
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.ServiceKey)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errWrongCRType)
	}

	cr.SetConditions(xpv1.Creating())

	r, err := c.servicekey.Create(ctx, cr.Spec.ForProvider, nil)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreate)
	}

	meta.SetExternalName(cr, r.GUID)

	return managed.ExternalCreation{
		ConnectionDetails: c.servicekey.GetConnectionDetails(ctx, r.GUID, cr.Spec.ForProvider.ConnectionDetailsAsJSON),
	}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	_, ok := mg.(*v1alpha1.ServiceKey)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errWrongCRType)
	}

	// Nothing to do, since none of the `ForProvider` parameters are updatable.
	return managed.ExternalUpdate{}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.ServiceKey)
	if !ok {
		return errors.New(errWrongCRType)
	}
	cr.SetConditions(xpv1.Deleting())

	if cr.Status.AtProvider.ID == nil {
		return nil
	}

	err := c.servicekey.Delete(ctx, *cr.Status.AtProvider.ID)
	if err != nil {
		return errors.Wrap(err, errDelete)
	}

	return nil
}
