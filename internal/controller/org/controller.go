package org

import (
	"context"

	"github.com/pkg/errors"
	"k8s.io/utils/ptr"

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
	pcv1beta1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/v1beta1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients"
	org "github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/org"
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
	errGet               = "cannot get " + resourceType + " in " + externalSystem
)

// Setup adds a controller that reconciles Org resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.Org_GroupKind)

	options := []managed.ReconcilerOption{
		managed.WithExternalConnecter(&connector{
			kube:  mgr.GetClient(),
			usage: resource.NewProviderConfigUsageTracker(mgr.GetClient(), &pcv1beta1.ProviderConfigUsage{}),
		}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
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
	usage *resource.ProviderConfigUsageTracker
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

	if err := c.usage.Track(ctx, mg.(resource.ModernManaged)); err != nil {
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

	external_name := meta.GetExternalName(cr)

	name := cr.Spec.ForProvider.Name
	// if name is not set, use the external name for backward compatibility
	if name == "" {
		name = external_name
	}

	o, err := org.GetByIDOrName(ctx, c.client, external_name, name)

	if err != nil {
		if clients.ErrorIsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}

		return managed.ExternalObservation{}, errors.Wrap(err, errGetResource)
	}

	org.LateInitialize(&cr.Spec.ForProvider, o)

	// set the external name to the GUID
	if external_name != o.GUID {
		meta.SetExternalName(cr, o.GUID)
		if err := c.kube.Update(ctx, cr); err != nil {
			return managed.ExternalObservation{}, errors.Wrap(err, errGet)
		}
	}

	cr.Status.AtProvider = org.GenerateObservation(o)

	if !ptr.Deref(cr.Status.AtProvider.Suspended, false) {
		cr.Status.SetConditions(xpv1.Available())
	}

	return managed.ExternalObservation{
		ResourceExists:   cr.Status.AtProvider.ID != nil,
		ResourceUpToDate: org.IsUpToDate(cr.Spec.ForProvider, o),
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
	// Do nothing, as Org is observe-only
	cr.SetConditions(xpv1.Deleting())
	return managed.ExternalDelete{}, nil
}
