package serviceroutebinding

import (
	"context"
	"errors"
	"fmt"
	"strings"

	cfclient "github.com/cloudfoundry/go-cfclient/v3/client"
	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	apisv1beta1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/v1beta1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/job"
	srb "github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/serviceroutebinding"
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
	errNoBindingReturned        = "no binding returned after creation"
	errParametersFromCF         = "cannot get parameters from " + resourceType + " in " + externalSystem + ": %w"
)

// Setup adds a controller that reconciles ServiceRouteBinding CR.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.ServiceRouteBinding_GroupKind)

	options := []managed.ReconcilerOption{
		managed.WithInitializers(),
		managed.WithExternalConnecter(&connector{
			kube:  mgr.GetClient(),
			usage: resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1beta1.ProviderConfigUsage{}),
		}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithPollInterval(o.PollInterval),
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
	usage *resource.ProviderConfigUsageTracker
}

// Connect establishes a client for ServiceRouteBinding operations.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	if _, ok := mg.(*v1alpha1.ServiceRouteBinding); !ok {
		return nil, errors.New(errWrongCRType)
	}

	if err := c.usage.Track(ctx, mg.(resource.ModernManaged)); err != nil {
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

// Disconnect implements the managed.ExternalClient interface
func (c *external) Disconnect(ctx context.Context) error {
	// No cleanup needed for Cloud Foundry client
	return nil
}

// Observe checks the current external state.
func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.ServiceRouteBinding)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errWrongCRType)
	}

	guid := meta.GetExternalName(cr)
	// check if the external-name exists, if yes the user wants to import an existing resource
	if guid == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}
	servicerouteBinding, err := srb.GetByID(ctx, e.srbClient, guid, cr.Spec.ForProvider)
	if isNotFoundError(err) {
		return managed.ExternalObservation{ResourceExists: false}, nil
	} else if err != nil {
		return managed.ExternalObservation{}, fmt.Errorf(errGet, err)
	}

	// detect if their should be parameters / if its a user-provided service instance (Is their a better way to detect this?)
	paramMap := &runtime.RawExtension{}
	if cr.Spec.ForProvider.Parameters.Raw != nil {
		paramMap, err = srb.GetParameters(ctx, e.srbClient, servicerouteBinding.GUID)
		if err != nil {
			return managed.ExternalObservation{}, fmt.Errorf(errExtractParams, err)
		}
	}

	srb.UpdateObservation(&cr.Status.AtProvider, servicerouteBinding, paramMap)

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

	routeGUID := cr.Spec.ForProvider.Route
	serviceInstanceGUID := cr.Spec.ForProvider.ServiceInstance
	if routeGUID == "" || serviceInstanceGUID == "" {
		return managed.ExternalCreation{}, fmt.Errorf(errCreate, fmt.Errorf(errMissingRelationshipGUIDs, routeGUID, serviceInstanceGUID))
	}

	// Get ParametersSecretRef if provided
	parameterFromSecret := runtime.RawExtension{}
	if cr.Spec.ForProvider.Parameters.Raw == nil && cr.Spec.ForProvider.ParametersSecretRef != nil {
		parameters, err := resolveParametersSecret(ctx, e.kube, cr.Spec.ForProvider)
		if err != nil {
			return managed.ExternalCreation{}, fmt.Errorf(errExtractParams, err)
		}
		parameterFromSecret = *parameters
	}

	binding, err := srb.Create(ctx, e.srbClient, cr.Spec.ForProvider, parameterFromSecret)
	if err != nil {
		return managed.ExternalCreation{}, fmt.Errorf(errCreate, err)
	} else if binding == nil {
		return managed.ExternalCreation{}, fmt.Errorf(errCreate, errors.New(errNoBindingReturned))
	}

	meta.SetExternalName(cr, binding.GUID)
	cr.SetConditions(xpv1.Creating())
	return managed.ExternalCreation{}, nil
}

// resolveParameters resolves ParametersSecretRef if set and returns the updated forProvider
func resolveParametersSecret(ctx context.Context, kube k8s.Client, forProvider v1alpha1.ServiceRouteBindingParameters) (*runtime.RawExtension, error) {
	if forProvider.ParametersSecretRef == nil {
		return nil, fmt.Errorf("ParametersSecretRef is not set")
	}

	jsonBytes, err := clients.ExtractSecret(ctx, kube, forProvider.ParametersSecretRef, "")
	if err != nil {
		return nil, err
	}

	if len(jsonBytes) == 0 {
		return nil, fmt.Errorf("no data found in secret")
	}

	return &runtime.RawExtension{Raw: jsonBytes}, nil
}

// Updates the external resource.
func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.ServiceRouteBinding)
	if !ok {
		return managed.ExternalUpdate{}, fmt.Errorf("managed resource is not a ServiceRouteBinding")
	}

	guid := meta.GetExternalName(cr)
	if guid == "" {
		return managed.ExternalUpdate{}, nil
	}

	// Update metadata (labels and annotations) - only supported fields for ServiceRouteBindings
	_, err := srb.Update(ctx, e.srbClient, guid, cr.Spec.ForProvider)
	if err != nil {
		return managed.ExternalUpdate{}, fmt.Errorf(errUpdate, err)
	}

	return managed.ExternalUpdate{}, nil
}

// Deletes the external resource.
func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1alpha1.ServiceRouteBinding)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errWrongCRType)
	}

	cr.SetConditions(xpv1.Deleting())

	err := srb.Delete(ctx, e.srbClient, meta.GetExternalName(cr))

	if isNotFoundError(err) {
		return managed.ExternalDelete{}, nil
	}
	if err != nil && !errors.Is(err, cfclient.AsyncProcessTimeoutError) {
		return managed.ExternalDelete{}, fmt.Errorf(errDelete, err)
	} else if err != nil {
		return managed.ExternalDelete{}, fmt.Errorf(errDelete, err)
	}
	return managed.ExternalDelete{}, nil
}

// handleObservationState processes the LastOperation state of a Service Route Binding
// and returns the appropriate ExternalObservation for Crossplane reconciliation.
//
// Note: Immutable fields (route, serviceInstance, parameters) are protected by CEL validation
// at the API level. Only metadata (labels/annotations) can be updated.
func handleObservationState(binding *cfresource.ServiceRouteBinding, cr *v1alpha1.ServiceRouteBinding) (managed.ExternalObservation, error) {
	state := binding.LastOperation.State
	typ := binding.LastOperation.Type

	switch state {
	case v1alpha1.LastOperationInitial, v1alpha1.LastOperationInProgress:
		cr.SetConditions(xpv1.Unavailable().WithMessage(binding.LastOperation.Description))
		return managed.ExternalObservation{
			ResourceExists:   true,
			ResourceUpToDate: true, // Do not update the resource while the last operation is in progress
		}, nil
	case v1alpha1.LastOperationFailed:
		cr.SetConditions(xpv1.Unavailable().WithMessage(binding.LastOperation.Description))
		// Service Route Bindings do not support updates, only create and delete operations
		return managed.ExternalObservation{
			ResourceExists:   typ != v1alpha1.LastOperationCreate, // Retry create if creation failed
			ResourceUpToDate: true,
		}, nil
	case v1alpha1.LastOperationSucceeded:
		if typ == v1alpha1.LastOperationDelete {
			return managed.ExternalObservation{ResourceExists: false, ResourceUpToDate: true}, nil
		}
		cr.SetConditions(xpv1.Available(), xpv1.ReconcileSuccess())

		// Check if metadata (labels/annotations) needs to be updated
		upToDate := isMetadataUpToDate(cr.Spec.ForProvider, binding)

		return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: upToDate}, nil
	}

	return managed.ExternalObservation{}, errors.New(errUnknownState)
}

// isMetadataUpToDate checks if labels and annotations match between desired state (spec) and actual state (CF).
// Returns true if metadata is up-to-date, false if an update is needed.
func isMetadataUpToDate(spec v1alpha1.ServiceRouteBindingParameters, binding *cfresource.ServiceRouteBinding) bool {
	// If no metadata in CF resource, check if spec wants to set any
	if binding.Metadata == nil {
		return spec.Labels == nil && spec.Annotations == nil
	}

	if !metadataMapEqual(spec.Labels, binding.Metadata.Labels) {
		return false
	}

	if !metadataMapEqual(spec.Annotations, binding.Metadata.Annotations) {
		return false
	}

	return true
}

// metadataMapEqual compares two metadata maps (labels or annotations).
func metadataMapEqual(desired, actual map[string]*string) bool {
	// check if both are nil/empty
	if len(desired) == 0 && len(actual) == 0 {
		return true
	}

	if len(desired) != len(actual) {
		return false
	}

	// Compare each key-value pair
	for key, desiredVal := range desired {
		actualVal, exists := actual[key]
		if !exists {
			return false
		}

		if (desiredVal == nil) != (actualVal == nil) {
			return false
		}
		if desiredVal != nil && actualVal != nil && *desiredVal != *actualVal {
			return false
		}
	}

	return true
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
