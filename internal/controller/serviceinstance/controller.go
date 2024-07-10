package serviceinstance

import (
	"context"
	"encoding/json"
	"time"

	"github.com/cloudfoundry-community/go-cfclient/v3/client"
	"github.com/nsf/jsondiff"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

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

	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/apis/service/v1alpha1"
	apisv1alpha1 "github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/apis/v1alpha1"
	apisv1beta1 "github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/apis/v1beta1"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/clients"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/clients/cfclient"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/clients/serviceinstance"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/features"
)

const (
	resourceType    = "ServiceInstance"
	externalSystem  = "Cloud Foundry"
	errTrackPCUsage = "cannot track ProviderConfig usage"
	errNewClient    = "cannot create a client for " + externalSystem
	errWrongCRType  = "managed resource is not a " + resourceType
	errUpdateCR     = "cannot update the managed resource"
	errGet          = "cannot get " + resourceType + " in " + externalSystem
	errCreate       = "cannot create " + resourceType + " in " + externalSystem
	errUpdate       = "cannot update " + resourceType + " in " + externalSystem
	errDelete       = "cannot delete " + resourceType + " in " + externalSystem
	errCleanFailed  = "cannot delete failed service instance"
	errSecret       = "cannot resolve secret reference"
)

// Setup adds a controller that reconciles ServiceInstance CR.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.ServiceInstanceGroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
	if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), apisv1alpha1.StoreConfigGroupVersionKind))
	}

	options := []managed.ReconcilerOption{
		managed.WithInitializers(&servicePlanInitializer{mgr.GetClient()}),
		managed.WithExternalConnecter(&connector{
			kube:        mgr.GetClient(),
			usage:       resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1beta1.ProviderConfigUsage{}),
			newClientFn: clients.CloudfoundryClientBuilder}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithTimeout(5 * time.Minute), // increase timeout for long-running operations
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithConnectionPublishers(cps...),
		managed.WithPollInterval(o.PollInterval),
	}

	if o.Features.Enabled(features.EnableBetaManagementPolicies) {
		options = append(options, managed.WithManagementPolicies())
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.ServiceInstanceGroupVersionKind),
		options...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&v1alpha1.ServiceInstance{}).
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
	if _, ok := mg.(*v1alpha1.ServiceInstance); !ok {
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
		kube:            c.kube,
		serviceinstance: serviceinstance.NewClient(cf),
	}, nil
}

// An external service observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	kube            k8s.Client
	serviceinstance *serviceinstance.Client
}

// Observe checks if the external resource exists and if it does, it observes it.
func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.ServiceInstance)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errWrongCRType)
	}

	// Check if the external resource exists
	r, err := c.serviceinstance.MatchSingle(ctx, cr.Spec.ForProvider)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errGet)
	}
	if r == nil {
		return managed.ExternalObservation{}, nil
	}
	// resource exists, set the external name
	meta.SetExternalName(cr, r.GUID)

	// Generate observation of the external resource
	externalState := serviceinstance.GenerateObservation(r)

	// Get the credentials from the external resource
	appliedCredentials, err := c.serviceinstance.GetServiceCredentials(ctx, r)
	if err != nil { // some services do not return credentials in the response, we use the stored credential in CR
		appliedCredentials = cr.Status.AtProvider.Credentials
	}
	externalState.Credentials = appliedCredentials

	// Update observation of the CR with the observed state
	cr.Status.AtProvider = externalState
	switch cr.Status.AtProvider.LastOperation.State {
	// If the last operation is in progress, set the CR to unavailable and signal that the reconciler should not update the resource
	case v1alpha1.LastOperationInitial, v1alpha1.LastOperationInProgress:
		cr.SetConditions(xpv1.Unavailable().WithMessage(r.LastOperation.Description))
		return managed.ExternalObservation{
			ResourceExists:   true,
			ResourceUpToDate: true, // Do not update the resource while the last operation is in progress
		}, nil
	// If the last operation failed, set the CR to unavailable and signal that the reconciler should retry the last operation
	case v1alpha1.LastOperationFailed:
		cr.SetConditions(xpv1.Unavailable().WithMessage(r.LastOperation.Description))
		return managed.ExternalObservation{
			ResourceExists:   r.LastOperation.Type != v1alpha1.LastOperationCreate, // set to false when the last operation is create, hence the reconciler will retry create
			ResourceUpToDate: r.LastOperation.Type != v1alpha1.LastOperationUpdate, // set to false when the last operation is update, hence the reconciler will retry update
		}, nil
	case v1alpha1.LastOperationSucceeded:
		cr.SetConditions(xpv1.Available())
		// Check if the credentials in the spec match the credentials in the external resource
		desiredCredentials, err := extractCredentialSpec(ctx, c.kube, cr.Spec.ForProvider)
		if err != nil {
			return managed.ExternalObservation{}, errors.Wrap(err, errSecret)
		}
		upToDate := serviceinstance.IsUpToDate(&cr.Spec.ForProvider, r) && jsonContain(appliedCredentials, desiredCredentials)
		return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: upToDate}, nil
	}

	// If the last operation is unknown, error out
	return managed.ExternalObservation{}, errors.New("unknown last operation state")
}

// Create attempts to create the external resource.
func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.ServiceInstance)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errWrongCRType)
	}

	// If the last operation is create and it failed, clean up the failed service instance before retry create
	if cr.Status.AtProvider.LastOperation.Type == v1alpha1.LastOperationCreate && cr.Status.AtProvider.LastOperation.State == v1alpha1.LastOperationFailed {
		err := c.serviceinstance.Delete(ctx, cr)
		if err != nil {
			return managed.ExternalCreation{}, errors.Wrap(err, errCleanFailed)
		}
	}

	cr.SetConditions(xpv1.Creating())

	// Extract the parameters or credentials from the spec as a json.RawMessage
	creds, err := extractCredentialSpec(ctx, c.kube, cr.Spec.ForProvider)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errSecret)
	}

	r, err := c.serviceinstance.Create(ctx, cr.Spec.ForProvider, creds)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreate)
	}

	// Set the external name of the CR
	meta.SetExternalName(cr, r.GUID)

	// Update the CR before updating the status so that the status update is not lost.
	if err = c.kube.Update(ctx, cr); err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errUpdateCR)
	}

	// Save credentials in the status of the CR
	cr.Status.AtProvider.Credentials = creds
	if err = c.kube.Status().Update(ctx, cr); err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errUpdateCR)
	}

	return managed.ExternalCreation{}, nil
}

// Update attempts to update the external resource to reflect the managed resource's desired state.
func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.ServiceInstance)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errWrongCRType)
	}

	if cr.Status.AtProvider.ID == nil {
		return managed.ExternalUpdate{}, errors.New(errUpdate)
	}

	creds, err := extractCredentialSpec(ctx, c.kube, cr.Spec.ForProvider)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errSecret)
	}

	if _, err := c.serviceinstance.Update(ctx, *cr.Status.AtProvider.ID, &cr.Spec.ForProvider, creds); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdate)
	}

	if err := c.kube.Update(ctx, cr); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateCR)
	}

	// Save credentials in the status of the CR
	if creds != nil {
		cr.Status.AtProvider.Credentials = creds
		if err := c.kube.Status().Update(ctx, cr); err != nil {
			return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateCR)
		}
	}

	return managed.ExternalUpdate{}, nil
}

// Delete attempts to delete the external resource.
func (c *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.ServiceInstance)
	if !ok {
		return errors.New(errWrongCRType)
	}
	cr.SetConditions(xpv1.Deleting())

	if err := c.serviceinstance.Delete(ctx, cr); err != nil {
		return errors.New(errDelete)
	}
	return nil
}

// extractSecret extracts parameters/credentials from a secret reference.
func extractSecret(ctx context.Context, kube k8s.Client, s *v1alpha1.SecretReference) (json.RawMessage, error) {
	if s == nil {
		return nil, nil
	}

	secret := &v1.Secret{}
	if err := kube.Get(ctx, types.NamespacedName{Namespace: s.Namespace, Name: s.Name}, secret); err != nil {
		return nil, errors.Wrap(err, errSecret)
	}

	// if key is specified, return data from the specific secret key
	if s.Key != nil {
		return secret.Data[*s.Key], nil
	}

	// if key is not specified, return all data from the secret
	cred := make(map[string]string)
	for key, value := range secret.Data {
		cred[key] = string(value)
	}
	buf, err := json.Marshal(cred)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

// extractCredentialSpec returns the parameters or credentials from the spec
func extractCredentialSpec(ctx context.Context, kube k8s.Client, spec v1alpha1.ServiceInstanceParameters) ([]byte, error) {
	if spec.Type == v1alpha1.ManagedService {
		if spec.JSONParams != nil {
			return []byte(*spec.JSONParams), nil
		}
		return extractSecret(ctx, kube, spec.ParamsSecretRef)
	}

	if spec.Type == v1alpha1.UserProvidedService {
		if spec.JSONCredentials != nil {
			return []byte(*spec.JSONCredentials), nil
		}
		return extractSecret(ctx, kube, spec.CredentialsSecretRef)
	}
	return nil, nil
}

// A servicePlanInitializer is expected to initialize the service plan of a ServiceInstance
type servicePlanInitializer struct {
	kube k8s.Client
}

// Initialize implements crossplane InitializeFn interface
func (s *servicePlanInitializer) Initialize(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.ServiceInstance)
	if !ok {
		return errors.New("Not a ServiceInstance")
	}
	if cr.Spec.ForProvider.Type != "managed" {
		return nil
	}

	// fallback on crossplane.io/external-data annotation for backward compatibility
	if cr.Spec.ForProvider.ServicePlan == nil {
		sp := struct {
			ServicePlan *v1alpha1.ServicePlan `json:"service_plan"`
		}{ServicePlan: &v1alpha1.ServicePlan{}}

		if data, ok := mg.GetAnnotations()["crossplane.io/external-data"]; ok {
			if err := json.Unmarshal([]byte(data), &sp); err == nil {
				cr.Spec.ForProvider.ServicePlan = sp.ServicePlan
			}
		}
	}

	// Already initialized, do nothing.
	// NOTE: Do we allow update service plan of existing service instance??
	if cr.Spec.ForProvider.ServicePlan != nil && cr.Spec.ForProvider.ServicePlan.ID != nil {
		return nil
	}

	cf, err := clients.CloudfoundryClientBuilder(ctx, s.kube, mg)
	if err != nil {
		return errors.Wrapf(err, "Cannot initialize service plan")
	}

	opt := client.NewServicePlanListOptions()
	opt.ServiceOfferingNames.EqualTo(*cr.Spec.ForProvider.ServicePlan.Offering)
	opt.Names.EqualTo(*cr.Spec.ForProvider.ServicePlan.Plan)

	// There must be exactly one matching service plan
	sp, err := cf.ServicePlans.Single(ctx, opt)
	if err != nil {
		return errors.Wrapf(err, "Cannot initialize service plan using serviceName/servicePlanName: %s:%s`", *cr.Spec.ForProvider.ServicePlan.Offering, *cr.Spec.ForProvider.ServicePlan.Plan)
	}

	cr.Spec.ForProvider.ServicePlan.ID = &sp.GUID

	return s.kube.Update(ctx, cr)
}

// jsonContain returns true if the first JSON message is a superset or identical to the second JSON message
func jsonContain(a, b []byte) bool {
	// if b is "{}", it is considered as empty
	if len(b) == 0 || string(b) == "{}" {
		return true
	}

	opt := jsondiff.DefaultConsoleOptions()
	diff, _ := jsondiff.Compare(a, b, &opt)
	return diff == jsondiff.FullMatch || diff == jsondiff.SupersetMatch
}
