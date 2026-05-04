package domain

import (
	"context"
	"fmt"

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

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	apisv1alpha1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/v1alpha1"
	pcv1beta1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/v1beta1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients"
	domain "github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/domain"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/job"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/org"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/features"

	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
)

const (
	resourceType     = "Domain"
	externalSystem   = "Cloud Foundry"
	errNotDomainKind = "managed resource is not of kind " + resourceType
	errNameRequired  = "name is required, please set the name attribute"
	errTrackUsage    = "cannot track usage"
	errGetClient     = "cannot create a client to talk to the API of " + externalSystem
	errCreate        = "cannot create " + externalSystem + " domain"
	errGet           = "cannot get " + resourceType + " in " + externalSystem
	errDelete        = "cannot delete " + resourceType
	errUpdate        = "cannot update " + resourceType
)

// Setup adds a controller that reconciles Domain resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.Domain_GroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
	if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), apisv1alpha1.StoreConfigGroupVersionKind))
	}

	options := []managed.ReconcilerOption{

		managed.WithExternalConnecter(&connector{
			kube:  mgr.GetClient(),
			usage: resource.NewProviderConfigUsageTracker(mgr.GetClient(), &pcv1beta1.ProviderConfigUsage{}),
		}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithConnectionPublishers(cps...),
		managed.WithInitializers(initializer{
			client: mgr.GetClient(),
		}),
	}

	if o.Features.Enabled(features.EnableBetaManagementPolicies) {
		options = append(options, managed.WithManagementPolicies())
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.Domain_GroupVersionKind),
		options...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&v1alpha1.Domain{}).
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
	if _, ok := mg.(*v1alpha1.Domain); !ok {
		return nil, errors.New(errNotDomainKind)
	}

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackUsage)
	}

	cf, err := clients.ClientFnBuilder(ctx, c.kube)(mg)
	if err != nil {
		return nil, errors.Wrap(err, errGetClient)
	}

	domainClient, jobClient := domain.NewClient(cf)
	return &external{client: domainClient, kube: c.kube, job: jobClient}, nil
}

// Disconnect implements the managed.ExternalClient interface
func (c *external) Disconnect(ctx context.Context) error {
	// No cleanup needed for Cloud Foundry client
	return nil
}

// DomainService defines the operations needed for Domain external-name handling.
type DomainService interface {
	FindDomainBySpec(ctx context.Context, spec v1alpha1.DomainParameters) (*cfresource.Domain, error)
	GetDomainByGUID(ctx context.Context, guid string) (*cfresource.Domain, error)
	Create(ctx context.Context, create *cfresource.DomainCreate) (*cfresource.Domain, error)
	Update(ctx context.Context, guid string, update *cfresource.DomainUpdate) (*cfresource.Domain, error)
	Delete(ctx context.Context, guid string) (string, error)
}

// An external is a managed.ExternalConnecter that is using the CloudFoundry API to observe and modify resources.
type external struct {
	client DomainService
	kube   k8s.Client
	job    job.Job
}

// Observe managed resource Domain
func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.Domain)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotDomainKind)
	}

	resourceLateInitialized, exists, err := c.resolveExternalName(ctx, cr)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	if !exists {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	guid := meta.GetExternalName(cr)

	if !clients.IsValidGUID(guid) {
		return managed.ExternalObservation{}, errors.New(
			fmt.Sprintf("external-name '%s' is not a valid GUID format", guid))
	}

	d, err := c.client.GetDomainByGUID(ctx, guid)
	if err != nil {
		if clients.ErrorIsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errGet)
	}

	if d == nil {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	cr.Status.AtProvider = domain.GenerateObservation(d)
	cr.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:          cr.Status.AtProvider.ID != nil,
		ResourceUpToDate:        domain.IsUpToDate(cr, cr.Spec.ForProvider, d),
		ResourceLateInitialized: resourceLateInitialized,
	}, nil
}

// resolveExternalName sets the external-name on the Domain CR if it is empty,
// by looking up the domain by spec.
// Returns (lateInitialized, exists, error).
func (c *external) resolveExternalName(ctx context.Context, cr *v1alpha1.Domain) (bool, bool, error) {
	if meta.GetExternalName(cr) != "" {
		return false, true, nil
	}

	d, err := c.client.FindDomainBySpec(ctx, cr.Spec.ForProvider)
	if err != nil {
		if clients.ErrorIsNotFound(err) {
			return false, false, nil
		}
		return false, false, errors.Wrap(err, errGet)
	}
	if d == nil {
		return false, false, nil
	}

	meta.SetExternalName(cr, d.GUID)
	return true, true, nil
}

// Create a managed resource Domain
func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.Domain)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotDomainKind)
	}

	cr.SetConditions(xpv1.Creating())

	o, err := c.client.Create(ctx, domain.GenerateCreate(cr, cr.Spec.ForProvider))
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

// Update managed resource Domain
func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.Domain)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotDomainKind)
	}

	guid := meta.GetExternalName(cr)
	if guid == "" {
		return managed.ExternalUpdate{}, nil
	}

	// rename resource
	if cr.Name != ptr.Deref(cr.Status.AtProvider.Name, "") {
		_, err := c.client.Update(ctx, *cr.Status.AtProvider.ID, domain.GenerateUpdate(cr, cr.Spec.ForProvider))
		if err != nil {
			return managed.ExternalUpdate{}, errors.Wrap(err, errUpdate)
		}
	}

	return managed.ExternalUpdate{}, nil
}

// Delete managed resource Domain
func (c *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1alpha1.Domain)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotDomainKind)
	}

	cr.SetConditions(xpv1.Deleting())

	if meta.GetExternalName(cr) == "" {
		return managed.ExternalDelete{}, nil
	}

	jobGUID, err := c.client.Delete(ctx, meta.GetExternalName(cr))
	if err != nil {
		if clients.ErrorIsNotFound(err) {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, errors.Wrap(err, errDelete)
	}

	return managed.ExternalDelete{}, job.PollJobComplete(ctx, c.job, jobGUID)
}

// initializer type implements the managed.Initializer interface
type initializer struct {
	client k8s.Client
}

// Initialize method resolves the references which are not resolved by
// the crossplane reconciler.
func (i initializer) Initialize(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.Domain)
	if !ok {
		return errors.New(errNotDomainKind)
	}

	// check if the name is already set
	if cr.Spec.ForProvider.Name == "" {
		// if name is not set, calculate name by domain and subdomain
		if cr.Spec.ForProvider.SubDomain == nil || cr.Spec.ForProvider.Domain == nil {
			return errors.New(errNameRequired) // if subdomain is not set
		}

		cr.Spec.ForProvider.Name = fmt.Sprintf("%s.%s", *cr.Spec.ForProvider.SubDomain, *cr.Spec.ForProvider.Domain)
	}
	// Resolve orgRef/orgSelector references so spec.Org is populated before Observe/Create
	if cr.Spec.ForProvider.OrgRef != nil || cr.Spec.ForProvider.OrgSelector != nil {
		return cr.ResolveReferences(ctx, i.client)
	}

	// If orgName is provided, resolve by orgName
	if cr.Spec.ForProvider.OrgName != nil {
		return org.ResolveByName(ctx, clients.ClientFnBuilder(ctx, i.client), mg)
	}

	return nil
}
