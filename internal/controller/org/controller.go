package org

import (
	"context"
	"fmt"

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
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/org"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/features"
)

const (
	resourceType         = "Organization"
	externalSystem       = "Cloud Foundry"
	errNotOrgKind        = "managed resource is not of kind " + resourceType
	errTrackUsage        = "cannot track usage"
	errGetProviderConfig = "cannot get ProviderConfig or resolve credential references"
	errGetClient         = "cannot create a client to talk to the API of" + externalSystem
	errGetResource       = "cannot get " + externalSystem + " organization according to the specified parameters"
	errCreate            = "cannot create " + externalSystem + " organization"
)

// Setup adds a controller that reconciles Org resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.Org_GroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
	if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), scv1alpha1.StoreConfigGroupVersionKind))
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
		managed.WithInitializers(),
	}

	if o.Features.Enabled(features.EnableBetaManagementPolicies) {
		options = append(options, managed.WithManagementPolicies())
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.Org_GroupVersionKind),
		options...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&v1alpha1.Organization{}).
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
	if _, ok := mg.(*v1alpha1.Organization); !ok {
		return nil, errors.New(errNotOrgKind)
	}

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackUsage)
	}

	cf, err := clients.ClientFnBuilder(ctx, c.kube)(mg)
	if err != nil {
		return nil, errors.Wrap(err, errGetClient)
	}

	return &external{client: org.NewClient(cf), kube: c.kube}, nil
}

// Disconnect implements the managed.ExternalClient interface
func (c *external) Disconnect(ctx context.Context) error {
	// No cleanup needed for Cloud Foundry client
	return nil
}

// An external is a managed.ExternalConnecter that is using the CloudFoundry API to observe and modify resources.
type external struct {
	client org.Client
	kube   k8s.Client
}

// Observe managed resource Org
func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.Organization)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotOrgKind)
	}

	resourceLateInitialized := false

	// ADR Step 1: Check if external-name is empty
	if meta.GetExternalName(cr) == "" {
		o, err := org.FindOrgBySpec(ctx, c.client, cr.Spec.ForProvider)
		if err != nil {
			return managed.ExternalObservation{}, errors.Wrap(err, errGetResource)
		}
		if o == nil {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		meta.SetExternalName(cr, o.GUID)
		resourceLateInitialized = true
	}

	guid := meta.GetExternalName(cr)

	// ADR Step 2: Validate GUID format
	if !clients.IsValidGUID(guid) {
		return managed.ExternalObservation{}, errors.New(fmt.Sprintf("external-name '%s' is not a valid GUID format", guid))
	}

	// ADR Step 3: Get by GUID
	o, err := org.GetOrgByGUID(ctx, c.client, guid)
	if err != nil {
		if clients.ErrorIsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errGetResource)
	}

	if o == nil {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	org.LateInitialize(&cr.Spec.ForProvider, o)
	cr.Status.AtProvider = org.GenerateObservation(o)

	if !ptr.Deref(cr.Status.AtProvider.Suspended, false) {
		cr.Status.SetConditions(xpv1.Available())
	}

	return managed.ExternalObservation{
		ResourceExists:          true,
		ResourceUpToDate:        org.IsUpToDate(cr.Spec.ForProvider, o),
		ResourceLateInitialized: resourceLateInitialized,
	}, nil
}

// Create a managed resource Org
func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.Organization)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotOrgKind)
	}

	o, err := c.client.Create(ctx, org.GenerateCreate(cr.Spec.ForProvider))
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

// Update managed resource Org
func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	_, ok := mg.(*v1alpha1.Organization)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotOrgKind)
	}

	// Do nothing, as Org is observe-only

	return managed.ExternalUpdate{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

// Delete managed resource Org
func (c *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1alpha1.Organization)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotOrgKind)
	}
	cr.SetConditions(xpv1.Deleting())
	if meta.GetExternalName(cr) == "" {
		return managed.ExternalDelete{}, nil
	}
	// Do nothing else, as Org is observe-only
	return managed.ExternalDelete{}, nil
}
