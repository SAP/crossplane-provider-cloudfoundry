package serviceinstance

import (
	"bytes"
	"context"
	"crypto/sha256"
	"time"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/google/uuid"
	"github.com/nsf/jsondiff"
	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	apisv1beta1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/v1beta1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/serviceinstance"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/space"
)

const (
	resourceType          = "ServiceInstance"
	externalSystem        = "Cloud Foundry"
	errTrackPCUsage       = "cannot track ProviderConfig usage"
	errNewClient          = "cannot create a client for " + externalSystem
	errWrongCRType        = "managed resource is not a " + resourceType
	errUpdateCR           = "cannot update the managed resource"
	errGet                = "cannot get " + resourceType + " in " + externalSystem
	errCreate             = "cannot create " + resourceType + " in " + externalSystem
	errUpdate             = "cannot update " + resourceType + " in " + externalSystem
	errDelete             = "cannot delete " + resourceType + " in " + externalSystem
	errCleanFailed        = "cannot delete failed service instance"
	errSecret             = "cannot resolve secret reference"
	errGetParameters      = "cannot get parameters of the service instance for drift detection. Please check this is supported or set enableParameterDriftDetection to false."
	errMissingServicePlan = "managed resource service instance requires a service plan"
)

// Setup adds a controller that reconciles ServiceInstance CR.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.ServiceInstance_GroupKind)

	options := []managed.ReconcilerOption{
		managed.WithExternalConnecter(&connector{
			kube:  mgr.GetClient(),
			usage: resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1beta1.ProviderConfigUsage{}),
		}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithTimeout(5 * time.Minute), // increase timeout for long-running operations
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithPollInterval(o.PollInterval),
		managed.WithInitializers(
			spaceInitializer{kube: mgr.GetClient()},
			servicePlanInitializer{kube: mgr.GetClient()},
		),
	}


	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.ServiceInstance_GroupVersionKind),
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
	kube  k8s.Client
	usage *resource.ProviderConfigUsageTracker
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

	if err := c.usage.Track(ctx, mg.(resource.ModernManaged)); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	cf, err := clients.ClientFnBuilder(ctx, c.kube)(mg)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &external{
		kube:            c.kube,
		serviceinstance: serviceinstance.NewClient(cf),
	}, nil
}

// Disconnect implements the managed.ExternalClient interface
func (c *external) Disconnect(ctx context.Context) error {
	// No cleanup needed for Cloud Foundry client
	return nil
}

// An external service observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	kube            k8s.Client
	serviceinstance *serviceinstance.Client
}

// Observe checks if the external resource exists and if it does, it observes it.
func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) { //nolint:gocyclo
	cr, ok := mg.(*v1alpha1.ServiceInstance)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errWrongCRType)
	}

	// Check if the external resource exists
	guid := meta.GetExternalName(cr)

	// Normal (non‑deletion) observe path.
	r, err := serviceinstance.GetByIDOrSpec(ctx, c.serviceinstance, guid, cr.Spec.ForProvider)
	if err != nil {
		if clients.ErrorIsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errGet)
	}
	if r == nil {
		return managed.ExternalObservation{}, nil
	}
	// resource exists, set/update the external name
	if guid != r.GUID {
		meta.SetExternalName(cr, r.GUID)
		if err := c.kube.Update(ctx, cr); err != nil {
			return managed.ExternalObservation{}, errors.Wrap(err, errUpdateCR)
		}
	}

	// Update atProvider from the retrieved the service instance
	serviceinstance.UpdateObservation(&cr.Status.AtProvider, r)

	// If the CR is marked for deletion we stop normal observe logic.
	// We report "resource exists" so Crossplane will call Delete() next.
	// (Delete() will handle a "not found" case safely, so we don't check again here.)
	if meta.WasDeleted(mg) {
		return managed.ExternalObservation{ResourceExists: true}, nil
	}

	switch r.LastOperation.State {
	case v1alpha1.LastOperationInitial, v1alpha1.LastOperationInProgress:
		// Set the CR to unavailable and signal that the reconciler should not update the resource
		cr.SetConditions(xpv1.Unavailable().WithMessage(r.LastOperation.Description))
		return managed.ExternalObservation{
			ResourceExists:   true,
			ResourceUpToDate: true, // Set to true so that the reconciler do not schedule another update while the last operation is in progress
		}, nil
	// If the last operation failed, set the CR to unavailable and signal that the reconciler should retry the last operation
	case v1alpha1.LastOperationFailed:
		// If the last operation failed, set the CR to unavailable and signal that the reconciler should retry the last operation
		cr.SetConditions(xpv1.Unavailable().WithMessage(r.LastOperation.Description))
		return managed.ExternalObservation{
			ResourceExists:   r.LastOperation.Type != v1alpha1.LastOperationCreate, // set to false when the last operation is create, hence the reconciler will retry create
			ResourceUpToDate: r.LastOperation.Type != v1alpha1.LastOperationUpdate, // set to false when the last operation is update, hence the reconciler will retry update
		}, nil
	case v1alpha1.LastOperationSucceeded:
		// If the last operation succeeded, set the CR to available
		cr.SetConditions(xpv1.Available())
		var credentialsUpToDate bool
		desiredCredentials, err := extractCredentialSpec(ctx, c.kube, cr.Spec.ForProvider)
		if err != nil {
			return managed.ExternalObservation{}, errors.Wrap(err, errSecret)
		}
		// If parameter drift detection is enable, get actual credentials from the service instance
		if cr.Spec.EnableParameterDriftDetection {
			// Get the parameters of the service instance for drift detection
			cred, err := c.serviceinstance.GetServiceCredentials(ctx, r)
			if err != nil {
				return managed.ExternalObservation{ResourceExists: true}, errors.Wrap(err, errGetParameters)
			}
			cr.Status.AtProvider.Credentials = iSha256(cred)
			credentialsUpToDate = jsonContain(cred, desiredCredentials)
		} else {
			desiredHash := iSha256(desiredCredentials)
			credentialsUpToDate = bytes.Equal(desiredHash, cr.Status.AtProvider.Credentials)
		}
		// Check if the credentials in the spec match the credentials in the external resource
		upToDate := credentialsUpToDate && serviceinstance.IsUpToDate(&cr.Spec.ForProvider, r)
		return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: upToDate}, nil
	default:
		// should never reach here
		cr.SetConditions(xpv1.Unavailable().WithMessage(r.LastOperation.Description))
		// If the last operation is unknown, error out
		return managed.ExternalObservation{}, errors.New("unknown last operation state")
	}
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

	// Save hash value of credentials in the status of the CR
	cr.Status.AtProvider.Credentials = iSha256(creds)
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

	if creds != nil {
		cr.Status.AtProvider.Credentials = iSha256(creds)
		if err := c.kube.Status().Update(ctx, cr); err != nil {
			return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateCR)
		}
	}

	return managed.ExternalUpdate{}, nil
}

// Delete attempts to delete the external resource.
func (c *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1alpha1.ServiceInstance)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errWrongCRType)
	}
	cr.SetConditions(xpv1.Deleting())

	if err := c.serviceinstance.Delete(ctx, cr); err != nil {
		return managed.ExternalDelete{}, errors.New(errDelete)
	}
	return managed.ExternalDelete{}, nil
}

// extractCredentialSpec returns the parameters or credentials from the spec
func extractCredentialSpec(ctx context.Context, kube k8s.Client, spec v1alpha1.ServiceInstanceParameters) ([]byte, error) {
	if spec.Type == v1alpha1.ManagedService {
		if spec.Parameters != nil {
			return spec.Parameters.Raw, nil
		}

		if spec.JSONParams != nil {
			return []byte(*spec.JSONParams), nil
		}

		if spec.ParametersSecretRef != nil {
			return clients.ExtractSecret(ctx, kube, spec.ParametersSecretRef.SecretReference, spec.ParametersSecretRef.Key)
		}
	}

	if spec.Type == v1alpha1.UserProvidedService {
		if spec.Credentials != nil {
			return spec.Credentials.Raw, nil
		}

		if spec.JSONCredentials != nil {
			return []byte(*spec.JSONCredentials), nil
		}
		if spec.CredentialsSecretRef != nil {
			return clients.ExtractSecret(ctx, kube, spec.CredentialsSecretRef.SecretReference, spec.CredentialsSecretRef.Key)
		}
	}
	return nil, nil
}

// jsonContain returns true if the first JSON message is a superset or identical to the second JSON message
func jsonContain(a, b []byte) bool {
	// if b is "{}", it is considered as empty
	if len(b) == 0 || string(b) == "{}" {
		return true
	}

	// if a is nil it is considered as intention to break reconciliation
	if a == nil {
		return true
	}

	opt := jsondiff.DefaultConsoleOptions()
	diff, _ := jsondiff.Compare(a, b, &opt)
	return diff == jsondiff.FullMatch || diff == jsondiff.SupersetMatch
}

type spaceInitializer struct {
	kube k8s.Client
}

// / Initialize implements the Initializer interface
func (c spaceInitializer) Initialize(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.ServiceInstance)
	if !ok {
		return errors.New(errWrongCRType)
	}

	if cr.Spec.ForProvider.SpaceRef != nil || cr.Spec.ForProvider.SpaceSelector != nil {
		return cr.ResolveReferences(ctx, c.kube)
	}

	return space.ResolveByName(ctx, clients.ClientFnBuilder(ctx, c.kube), mg)
}

// A servicePlanInitializer is expected to initialize the service plan of a ServiceInstance
type servicePlanInitializer struct {
	kube k8s.Client
}

// Initialize implements crossplane InitializeFn interface
func (s servicePlanInitializer) Initialize(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.ServiceInstance)
	if !ok {
		return errors.New(errWrongCRType)
	}
	if cr.Spec.ForProvider.Type != "managed" {
		return nil
	}

	if cr.Spec.ForProvider.ServicePlan != nil {
		// When ServicePlan is set we populate the service
		// plan ID based on the external resource GUID.
		cf, err := clients.ClientFnBuilder(ctx, s.kube)(mg)
		if err != nil {
			return errors.Wrapf(err, errNewClient)
		}

		opt := client.NewServicePlanListOptions()
		if cr.Spec.ForProvider.ServicePlan.Offering != nil {
			opt.ServiceOfferingNames.EqualTo(*cr.Spec.ForProvider.ServicePlan.Offering)
		}
		if cr.Spec.ForProvider.ServicePlan.Plan != nil {
			opt.Names.EqualTo(*cr.Spec.ForProvider.ServicePlan.Plan)
		}
		sp, err := cf.ServicePlans.Single(ctx, opt)
		if err != nil {
			return errors.Wrapf(err, "Cannot initialize service plan using serviceName/servicePlanName: %s:%s`", *cr.Spec.ForProvider.ServicePlan.Offering, *cr.Spec.ForProvider.ServicePlan.Plan)
		}

		cr.Spec.ForProvider.ServicePlan.ID = &sp.GUID

		return s.kube.Update(ctx, cr)
	}

	// Service plan is not set
	guid := meta.GetExternalName(cr)

	if _, err := uuid.Parse(guid); err == nil {
		// We have a valid external-name annotation
		return nil
	}

	// No valid external-name annotation
	return errors.New(errMissingServicePlan)
}

// Small wrapper around sha256.Sum256()
// info: if creds == nil, it will result in a hash value anyway (e3b0c44298...).
// This should not be a security problem.
func iSha256(data []byte) []byte {
	if len(data) == 0 || string(data) == "{}" {
		return nil
	}
	s := sha256.Sum256(data)
	return s[:]
}
