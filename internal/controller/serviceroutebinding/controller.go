package serviceroutebinding

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/job"
	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/connection"
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	ctrl "sigs.k8s.io/controller-runtime"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	apisv1alpha1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/v1alpha1"
	apisv1beta1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/v1beta1"
	srb "github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/serviceroutebinding"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/features"
	cfclient "github.com/cloudfoundry/go-cfclient/v3/client"
)

// using this client https://pkg.go.dev/github.com/cloudfoundry/go-cfclient/v3@v3.0.0-alpha.12/client#ServiceRouteBindingClient

const (
	resourceType                = "ServiceRouteBinding"
	externalSystem              = "Cloud Foundry"
	errTrackPCUsage             = "cannot track ProviderConfig usage: %w"
	errNewClient                = "cannot create a client for " + externalSystem + ": %w"
	errWrongCRType              = "managed resource is not a " + resourceType
	errGet                      = "cannot get " + resourceType + " in " + externalSystem + ": %w"
	errFind                     = "cannot find " + resourceType + " in " + externalSystem
	errCreate                   = "cannot create " + resourceType + " in " + externalSystem + ": %w"
	errUpdate                   = "cannot update " + resourceType + " in " + externalSystem + ": %w"
	errDelete                   = "cannot delete " + resourceType + " in " + externalSystem + ": %w"
	errUpdateStatus             = "cannot update status after retiring binding: %w"
	errExtractParams            = "cannot extract specified parameters: %w"
	errUnknownState             = "unknown last operation state for " + resourceType + " in " + externalSystem
	errMissingRelationshipGUIDs = "missing relationship GUIDs (route=%q serviceInstance=%q)"
)

// Setup adds a controller that reconciles ServiceRouteBinding CR.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.ServiceRouteBinding_GroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
	if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), apisv1alpha1.StoreConfigGroupVersionKind))
	}

	options := []managed.ReconcilerOption{
		managed.WithInitializers(),
		managed.WithExternalConnecter(&connector{
			kube:  mgr.GetClient(),
			usage: resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1beta1.ProviderConfigUsage{}),
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
		resource.ManagedKind(v1alpha1.ServiceRouteBinding_GroupVersionKind),
		options...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&v1alpha1.ServiceRouteBinding{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector is expected to produce an external client when its Connect method
// is called.
type connector struct {
	kube  k8s.Client
	usage resource.Tracker
}

// Connect establishes a client for ServiceRouteBinding operations.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	if _, ok := mg.(*v1alpha1.ServiceRouteBinding); !ok {
		return nil, errors.New(errWrongCRType)
	}

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, fmt.Errorf(errTrackPCUsage, err)
	}

	cf, err := clients.ClientFnBuilder(ctx, c.kube)(mg)
	if err != nil {
		return nil, fmt.Errorf(errNewClient, err)
	}

	client := srb.NewClient(cf)

	ext := &external{
		kube:      c.kube,
		srbClient: client,
		job:       cf.Jobs,
	}
	return ext, nil
}

// external implements the managed.ExternalClient interface for ServiceRouteBinding.
type external struct {
	kube      k8s.Client
	srbClient srb.ServiceRouteBinding
	job       job.Job
}

// Observe checks the current external state.
func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.ServiceRouteBinding)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errWrongCRType)
	}

	guid := meta.GetExternalName(cr)
	servicerouteBinding, err := srb.GetByIDOrSearch(ctx, e.srbClient, guid, cr.Spec.ForProvider)
	if isNotFoundError(err) {
		return managed.ExternalObservation{ResourceExists: false}, nil
	} else if err != nil {
		return managed.ExternalObservation{}, fmt.Errorf(errGet, err)
	}
	// maybe set external name if not exists/ is this a good practice?
	//meta.SetExternalName(cr, binding.GUID)

	srb.UpdateObservation(&cr.Status.AtProvider, servicerouteBinding)

	obs, herr := handleObservationState(servicerouteBinding, cr)
	if herr != nil {
		return managed.ExternalObservation{}, herr
	}
	return obs, nil
}

// Creates the external resource.
func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.ServiceRouteBinding)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errWrongCRType)
	}

	// check if allready exists
	if existing := meta.GetExternalName(cr); existing != "" {
		return managed.ExternalCreation{}, nil
	}

	routeGUID := cr.Spec.ForProvider.Relationships.Route.GUID
	serviceInstanceGUID := cr.Spec.ForProvider.Relationships.ServiceInstance.GUID
	if routeGUID == "" || serviceInstanceGUID == "" {
		return managed.ExternalCreation{}, fmt.Errorf(errCreate, fmt.Errorf(errMissingRelationshipGUIDs, routeGUID, serviceInstanceGUID))
	}

	binding, err := srb.Create(ctx, e.srbClient, cr.Spec.ForProvider)
	if err != nil {
		return managed.ExternalCreation{}, fmt.Errorf(errCreate, err)
	}

	if binding != nil {
		meta.SetExternalName(cr, binding.GUID)
	}
	cr.SetConditions(xpv1.Creating())
	return managed.ExternalCreation{}, nil
}

// Updates the external resource.
func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	_, ok := mg.(*v1alpha1.ServiceRouteBinding)
	if !ok {
		return managed.ExternalUpdate{}, fmt.Errorf("managed resource is not a ServiceRouteBinding")
	}
	// currently not implemented, since CF only support update of labels/annotations

	return managed.ExternalUpdate{}, nil
}

// Deletes the external resource.
func (e *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.ServiceRouteBinding)
	if !ok {
		return errors.New(errWrongCRType)
	}

	cr.SetConditions(xpv1.Deleting())

	guid := meta.GetExternalName(cr)
	if guid == "" {
		return nil
	}

	err := srb.Delete(ctx, e.srbClient, cr.Status.AtProvider.GUID)

	if isNotFoundError(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf(errDelete, err)
	}
	if !errors.Is(err, cfclient.AsyncProcessTimeoutError) {
		return fmt.Errorf(errDelete, err)
	}
	return nil
}

func handleObservationState(binding *cfresource.ServiceRouteBinding, cr *v1alpha1.ServiceRouteBinding) (managed.ExternalObservation, error) {
	state := binding.LastOperation.State
	typ := binding.LastOperation.Type

	switch state {
	case v1alpha1.LastOperationInitial, v1alpha1.LastOperationInProgress:
		cr.SetConditions(xpv1.Unavailable().WithMessage(binding.LastOperation.Description))
		return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, nil
	case v1alpha1.LastOperationFailed:
		cr.SetConditions(xpv1.Unavailable().WithMessage(binding.LastOperation.Description))
		if typ == v1alpha1.LastOperationCreate {
			return managed.ExternalObservation{ResourceExists: false, ResourceUpToDate: false}, nil
		}
		if typ == v1alpha1.LastOperationUpdate {
			return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: false}, nil
		}
		if typ == v1alpha1.LastOperationDelete {
			return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, nil
		}
		return managed.ExternalObservation{}, fmt.Errorf("%s: unknown failed operation type %q", errUnknownState, typ)
	case v1alpha1.LastOperationSucceeded:
		if typ == v1alpha1.LastOperationDelete {
			cr.SetConditions(xpv1.Deleting())
			return managed.ExternalObservation{ResourceExists: false, ResourceUpToDate: true}, nil
		}
		cr.SetConditions(xpv1.Available())
		return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, nil
	}

	return managed.ExternalObservation{}, errors.New(errUnknownState)
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, cfclient.ErrNoResultsReturned) || errors.Is(err, cfclient.ErrExactlyOneResultNotReturned) {
		return true
	}
	msg := err.Error()
	if strings.Contains(msg, "CF-ResourceNotFound") {
		return true
	}
	if strings.Contains(strings.ToLower(msg), "service route binding not found") {
		return true
	}
	return false
}
