package spacequota

import (
	"context"
	"slices"

	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reference"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/pkg/errors"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/google/go-cmp/cmp"

	resources "github.com/SAP/crossplane-provider-cloudfoundry/apis/resources"
	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	apisv1beta1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/v1beta1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/spacequota"
)

const (
	errTrackPCUsage      = "cannot track ProviderConfig usage"
	errGetProviderConfig = "cannot get ProviderConfig or resolve credential references"
	errNewClient         = "cannot create new client"
	errUnexpectedObject  = "managed resource is not a cloudfoundry SpaceQuota"
	errGet               = "cannot get cloudfoundry SpaceQuota"
	errResolveReferences = "cannot resolve references"
	errCreate            = "cannot create cloudfoundry SpaceQuota"
	errUpdate            = "cannot update cloudfoundry SpaceQuota"
	errUpdateOrg         = "cannot update org of cloudfoundry SpaceQuota"
	errDelete            = "cannot delete cloudfoundry SpaceQuota"
)

// Setup adds a controller that reconciles space quota managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.SpaceQuota_GroupKind)
	options := []managed.ReconcilerOption{

		managed.WithExternalConnecter(&connector{
			kube:  mgr.GetClient(),
			usage: resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1beta1.ProviderConfigUsage{}),
		}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithPollInterval(o.PollInterval),
		managed.WithInitializers(initializer{
			client: mgr.GetClient(),
		}),
	}


	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.SpaceQuota_GroupVersionKind),
		options...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&v1alpha1.SpaceQuota{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector is expected to produce an ExternalClient when its
// Connect method is called.
type connector struct {
	kube  k8s.Client
	usage *resource.ProviderConfigUsageTracker
}

// ResolveReferences resolves the references in the managed resources
// using the crossplane reference resolution algorithm.
func ResolveReferences(ctx context.Context, mg *v1alpha1.SpaceQuota, client k8s.Reader) error {
	r := reference.NewAPIResolver(client, mg)

	var rsp reference.ResolutionResponse
	var err error

	rsp, err = r.Resolve(ctx, reference.ResolutionRequest{
		CurrentValue: reference.FromPtrValue(mg.Spec.ForProvider.Org),
		Extract:      resources.ExternalID(),
		Reference:    clients.NamespacedRefToRef(mg.Spec.ForProvider.OrgRef),
		Selector:     clients.NamespacedSelectorToSelector(mg.Spec.ForProvider.OrgSelector),
		Namespace:    mg.GetNamespace(),
		To: reference.To{
			List:    &v1alpha1.OrganizationList{},
			Managed: &v1alpha1.Organization{},
		},
	})
	if err != nil {
		return errors.Wrap(err, "mg.Spec.ForProvider.Organization")
	}
	mg.Spec.ForProvider.Org = reference.ToPtrValue(rsp.ResolvedValue)
	mg.Spec.ForProvider.OrgRef = clients.RefToNamespacedRef(rsp.ResolvedReference)

	rsp, err = r.Resolve(ctx, reference.ResolutionRequest{
		CurrentValue: reference.FromPtrValue(mg.Spec.InitProvider.Org),
		Extract:      resources.ExternalID(),
		Reference:    clients.NamespacedRefToRef(mg.Spec.InitProvider.OrgRef),
		Selector:     clients.NamespacedSelectorToSelector(mg.Spec.InitProvider.OrgSelector),
		Namespace:    mg.GetNamespace(),
		To: reference.To{
			List:    &v1alpha1.OrganizationList{},
			Managed: &v1alpha1.Organization{},
		},
	})
	if err != nil {
		return errors.Wrap(err, "mg.Spec.InitProvider.Organization")
	}
	mg.Spec.InitProvider.Org = reference.ToPtrValue(rsp.ResolvedValue)
	mg.Spec.InitProvider.OrgRef = clients.RefToNamespacedRef(rsp.ResolvedReference)

	return nil
}

// initializer type implements the managed.Initializer interface
type initializer struct {
	client k8s.Reader
}

// Initialize method resolves the references which are not resolved by
// the crossplane reconciler.
func (i initializer) Initialize(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.SpaceQuota)
	if !ok {
		return errors.New(errUnexpectedObject)
	}

	return ResolveReferences(ctx, cr, i.client)
}

// isUpToDate function checks whether an managed resource is up to
// date by observing an external resource.
//
//nolint:gocyclo
func isUpToDate(ctx context.Context,
	cr *v1alpha1.SpaceQuota,
	resp *cfresource.SpaceQuota) (bool, error) {
	spec := &cr.Spec.ForProvider
	if v := spec.AllowPaidServicePlans; v != nil {
		if *v != resp.Services.PaidServicesAllowed {
			return false, nil
		}
	}
	if v := spec.InstanceMemory; v != nil {
		vInt := int(*v)
		if !ptr.Equal(&vInt, resp.Apps.PerProcessMemoryInMB) {
			return false, nil
		}
	}
	if v := spec.Name; v != nil {
		if *v != resp.Name {
			return false, nil
		}
	}
	if v := spec.Org; v != nil {
		if *v != resp.Relationships.Organization.Data.GUID {
			return false, errors.New(errUpdateOrg)
		}
	}
	if len(spec.Spaces) != len(resp.Relationships.Spaces.Data) {
		return false, nil
	}
	specSpaces := make([]string, len(spec.Spaces))
	respSpaces := make([]string, len(spec.Spaces))
	for i := range specSpaces {
		specSpaces[i] = *spec.Spaces[i]
		respSpaces[i] = resp.Relationships.Spaces.Data[i].GUID
	}
	slices.Sort(specSpaces)
	slices.Sort(respSpaces)
	if slices.Compare(specSpaces, respSpaces) != 0 {
		return false, nil
	}
	if v := spec.TotalAppInstances; v != nil {
		vInt := int(*v)
		if !ptr.Equal(&vInt, resp.Apps.TotalInstances) {
			return false, nil
		}
	}
	if v := spec.TotalAppLogRateLimit; v != nil {
		vInt := int(*v)
		if !ptr.Equal(&vInt, resp.Apps.LogRateLimitInBytesPerSecond) {
			return false, nil
		}
	}
	if v := spec.TotalAppTasks; v != nil {
		vInt := int(*v)
		if !ptr.Equal(&vInt, resp.Apps.PerAppTasks) {
			return false, nil
		}
	}
	if v := spec.TotalMemory; v != nil {
		vInt := int(*v)
		if !ptr.Equal(&vInt, resp.Apps.TotalMemoryInMB) {
			return false, nil
		}
	}
	if v := spec.TotalRoutePorts; v != nil {
		vInt := int(*v)
		if !ptr.Equal(&vInt, resp.Routes.TotalReservedPorts) {
			return false, nil
		}
	}
	if v := spec.TotalRoutes; v != nil {
		vInt := int(*v)
		if !ptr.Equal(&vInt, resp.Routes.TotalRoutes) {
			return false, nil
		}
	}
	if v := spec.TotalServiceKeys; v != nil {
		vInt := int(*v)
		if !ptr.Equal(&vInt, resp.Services.TotalServiceKeys) {
			return false, nil
		}
	}
	if v := spec.TotalServices; v != nil {
		vInt := int(*v)
		if !ptr.Equal(&vInt, resp.Services.TotalServiceInstances) {
			return false, nil
		}
	}
	return true, nil
}

// // Connect typically produces an ExternalClient by:
// 1. Tracking that the managed resource is using a ProviderConfig.
// 2. Getting the managed resource's ProviderConfig.
// 3. Getting the credentials specified by the ProviderConfig.
// 4. Using the credentials to form a client.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	if _, ok := mg.(*v1alpha1.SpaceQuota); !ok {
		return nil, errors.New(errUnexpectedObject)
	}

	if err := c.usage.Track(ctx, mg.(resource.ModernManaged)); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	cf, err := clients.ClientFnBuilder(ctx, c.kube)(mg)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}
	return &external{client: spacequota.NewClient(cf), kube: c.kube, isUpToDate: isUpToDate}, nil
}

// Disconnect implements the managed.ExternalClient interface
func (c *external) Disconnect(ctx context.Context) error {
	// No cleanup needed for Cloud Foundry client
	return nil
}

// An ExternalClient observes, then either creates, updates, or
// deletes an external resource to ensure it reflects the managed
// resource's desired state.
type external struct {
	kube       k8s.Client
	client     spacequota.SpaceQuotaClient
	isUpToDate func(context.Context,
		*v1alpha1.SpaceQuota,
		*cfresource.SpaceQuota) (bool, error)
}

// Observe generates observation for a space
func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.SpaceQuota)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errUnexpectedObject)
	}

	if meta.GetExternalName(cr) == "" {
		return managed.ExternalObservation{}, nil
	}

	resp, err := e.client.Get(ctx, meta.GetExternalName(cr))
	if err != nil {
		if clients.ErrorIsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{ResourceExists: false}, errors.Wrap(err, errGet)
	}

	currentSpec := cr.Spec.ForProvider.DeepCopy()
	GenerateSpaceQuota(resp).Status.AtProvider.DeepCopyInto(&cr.Status.AtProvider)
	cr.SetConditions(xpv1.Available())

	upToDate := true
	if !meta.WasDeleted(cr) { // There is no need to run isUpToDate if the resource is deleted
		upToDate, err = e.isUpToDate(ctx, cr, resp)
		if err != nil {
			return managed.ExternalObservation{}, errors.Wrap(err, "isUpToDate check failed")
		}
	}

	return managed.ExternalObservation{
		ResourceExists:          true,
		ResourceUpToDate:        upToDate,
		ResourceLateInitialized: !cmp.Equal(&cr.Spec.ForProvider, currentSpec),
	}, nil
}

// Create creates a space quota
func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.SpaceQuota)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errUnexpectedObject)
	}

	cr.SetConditions(xpv1.Creating())

	resp, err := c.client.Create(ctx, GenerateCreateSpaceQuota(cr))
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreate)
	}

	meta.SetExternalName(cr, resp.GUID)

	GenerateSpaceQuota(resp).Status.AtProvider.DeepCopyInto(&cr.Status.AtProvider)

	return managed.ExternalCreation{}, nil
}

// Update updates a space quota
func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.SpaceQuota)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errUnexpectedObject)
	}

	if cr.Status.AtProvider.ID == nil {
		return managed.ExternalUpdate{}, errors.New(errUpdate)
	}

	resp, err := c.client.Update(ctx, *cr.Status.AtProvider.ID, GenerateUpdateSpaceQuota(cr))
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdate)
	}

	sStatus := getSpaceStatus(cr)
	if toCreate := sStatus.toCreate(); len(toCreate) > 0 {
		_, err := c.client.Apply(ctx, *cr.Status.AtProvider.ID, toCreate)
		if err != nil {
			return managed.ExternalUpdate{}, errors.Wrap(err, errUpdate)
		}
	}
	if toDelete := sStatus.toDelete(); len(toDelete) > 0 {
		for i := range toDelete {
			err := c.client.Remove(ctx, *cr.Status.AtProvider.ID, toDelete[i])
			if err != nil {
				return managed.ExternalUpdate{}, errors.Wrap(err, errUpdate)
			}
		}
	}
	GenerateSpaceQuota(resp).Status.AtProvider.DeepCopyInto(&cr.Status.AtProvider)
	return managed.ExternalUpdate{}, nil
}

// Delete deletes a space quota
func (c *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1alpha1.SpaceQuota)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errUnexpectedObject)
	}
	cr.SetConditions(xpv1.Deleting())

	// assert that ID is set
	if cr.Status.AtProvider.ID == nil {
		return managed.ExternalDelete{}, errors.New(errDelete)
	}

	for i := range cr.Status.AtProvider.Spaces {
		err := c.client.Remove(ctx, *cr.Status.AtProvider.ID, *cr.Status.AtProvider.Spaces[i])
		if err != nil {
			return managed.ExternalDelete{}, errors.Wrap(err, errDelete)
		}
	}
	_, err := c.client.Delete(ctx, *cr.Status.AtProvider.ID)
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, errDelete)
	}

	return managed.ExternalDelete{}, nil
}
