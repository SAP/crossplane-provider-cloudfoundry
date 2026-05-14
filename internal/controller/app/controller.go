package app

import (
	"bytes"
	"context"
	"encoding/json"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/connection"
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/docker/cli/cli/config/configfile"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	scv1alpha1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/v1alpha1"
	pcv1beta1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/v1beta1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/app"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/space"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/features"
)

var (
	resourceKind       = v1alpha1.App_Kind
	errWrongKind       = "Wrong resource kind (expected " + resourceKind + " resource)"
	errTrackUsage      = "Cannot track usage"
	errConnect         = "Cannot connect to Cloud Foundry"
	errObserveResource = "Cannot observe" + resourceKind + " by ID or using forProvider spec"
	errCreateResource  = "Cannot create " + resourceKind + " resource in Cloud Foundry"
	errUpdateResource  = "Cannot update " + resourceKind + " in Cloud Foundry"
	errDeleteResource  = "Cannot delete " + resourceKind + " in Cloud Foundry"
	errSecret          = "Cannot extract credentials from secret"
)

// Setup adds a controller that reconciles App resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(resourceKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
	if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), scv1alpha1.StoreConfigGroupVersionKind))
	}

	options := []managed.ReconcilerOption{
		managed.WithExternalConnecter(
			&connector{kube: mgr.GetClient(),
				usage: resource.NewProviderConfigUsageTracker(mgr.GetClient(), &pcv1beta1.ProviderConfigUsage{}),
			}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithConnectionPublishers(cps...),
		managed.WithInitializers(&spaceInitializer{
			kube: mgr.GetClient(),
		}),
	}

	if o.Features.Enabled(features.EnableBetaManagementPolicies) {
		options = append(options, managed.WithManagementPolicies())
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.App_GroupVersionKind),
		options...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&v1alpha1.App{}).
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
	if _, ok := mg.(*v1alpha1.App); !ok {
		return nil, errors.New(errWrongKind)
	}

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackUsage)
	}

	cf, err := clients.ClientFnBuilder(ctx, c.kube)(mg)
	if err != nil {
		return nil, errors.Wrap(err, errConnect)
	}

	return &external{
		client: app.NewAppClient(cf),
		kube:   c.kube,
	}, nil
}

// An external provide clients to operate both Kubernetes resources and Cloud Foundry resources.
type external struct {
	client *app.Client
	kube   k8s.Client
}

// Observe managed resource
func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.App)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errWrongKind)
	}

	guid := meta.GetExternalName(cr)
	res, err := c.client.GetByIDOrSpec(ctx, guid, cr.Spec.ForProvider)
	if err != nil {
		if clients.ErrorIsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}

		return managed.ExternalObservation{}, errors.Wrap(err, errObserveResource)
	}

	lateInitialized := false

	// Update external_name if it is not set or different
	if guid != res.GUID {
		meta.SetExternalName(cr, res.GUID)
		lateInitialized = true
	}

	// Preserve previously observed routes so they survive a transient
	// failure from the Routes API.
	prevRoutes := cr.Status.AtProvider.Routes

	// Update the status of the resource
	cr.Status.AtProvider = app.GenerateObservation(res)
	appManifest, err := c.client.GenerateManifest(ctx, res.GUID)
	if err == nil {
		cr.Status.AtProvider.AppManifest = appManifest
	}

	// Fetch routes for the application. On success, update the status with
	// the fresh data; on error, restore the previously observed routes so
	// that a transient CF API failure does not erase known route information.
	if routes, err := c.client.FetchRoutes(ctx, res.GUID); err == nil {
		cr.Status.AtProvider.Routes = routes
	} else {
		cr.Status.AtProvider.Routes = prevRoutes
		klog.Warningf("failed to fetch routes for app %q, preserving previous observations: %v", res.GUID, err)
	}

	// Set condition according to app State
	switch cr.Status.AtProvider.State {
	case "STARTED":
		cr.SetConditions(xpv1.Available())
	case "STOPPED":
		cr.SetConditions(xpv1.Unavailable())
	default:
		cr.SetConditions(xpv1.Unavailable())
	}

	isUpToDate, err := app.IsUpToDate(cr.Spec.ForProvider, cr.Status.AtProvider)
	if err != nil {
		return managed.ExternalObservation{}, err
	}

	return managed.ExternalObservation{
		ResourceExists:          true,
		ResourceUpToDate:        isUpToDate,
		ResourceLateInitialized: lateInitialized,
	}, nil
}

// Create managed resource
func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.App)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errWrongKind)
	}

	dockerCredentials, err := getDockerCredential(ctx, c.kube, cr.Spec.ForProvider)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errSecret)
	}

	cr.SetConditions(xpv1.Creating())

	application, err := c.client.CreateAndPush(ctx, cr.Spec.ForProvider, dockerCredentials)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateResource)
	}
	meta.SetExternalName(cr, application.GUID)

	return managed.ExternalCreation{}, nil
}

// Update managed resource
func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.App)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errWrongKind)
	}

	guid := meta.GetExternalName(cr)
	if _, err := uuid.Parse(guid); err != nil {
		return managed.ExternalUpdate{}, errors.New(errUpdateResource + ": No valid GUID found for the App")
	}

	changes, err := app.DetectChanges(cr.Spec.ForProvider, cr.Status.AtProvider)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateResource+": Failed to detect changes")
	}

	if changes.HasField("docker_image") {
		if err := c.updateDockerImage(ctx, guid, cr); err != nil {
			return managed.ExternalUpdate{}, err
		}
	}

	if changes.HasField("environment") {
		if err := c.updateEnvVars(ctx, guid, cr); err != nil {
			return managed.ExternalUpdate{}, err
		}
	}

	if changes.HasOtherChanges("docker_image", "environment") {
		_, err := c.client.Update(ctx, guid, cr.Spec.ForProvider)
		if err != nil {
			return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateResource)
		}
	}

	return managed.ExternalUpdate{}, nil
}

// updateDockerImage pushes a new docker image for the app.
func (c *external) updateDockerImage(ctx context.Context, guid string, cr *v1alpha1.App) error {
	dockerCredentials, err := getDockerCredential(ctx, c.kube, cr.Spec.ForProvider)
	if err != nil {
		return errors.Wrap(err, errSecret)
	}
	_, err = c.client.UpdateAndPush(ctx, guid, cr.Spec.ForProvider, dockerCredentials)
	return errors.Wrap(err, errUpdateResource)
}

// updateEnvVars updates the environment variables of the app via the CF API directly.
// It sets new/updated vars and sends nil for vars that exist in CF but were removed from spec.
func (c *external) updateEnvVars(ctx context.Context, guid string, cr *v1alpha1.App) error {
	// Build desired env vars from spec
	envVars := map[string]*string{}
	if cr.Spec.ForProvider.Environment != nil && cr.Spec.ForProvider.Environment.Raw != nil {
		raw := map[string]string{}
		if err := json.Unmarshal(cr.Spec.ForProvider.Environment.Raw, &raw); err != nil {
			return errors.Wrap(err, errUpdateResource)
		}
		for k, v := range raw {
			v := v
			envVars[k] = &v
		}
	}
	// Get current CF env vars and send nil for any that are no longer in spec
	currentVars, err := c.client.GetEnvironmentVariables(ctx, guid)
	if err != nil {
		return errors.Wrap(err, errUpdateResource)
	}
	for k := range currentVars {
		if _, exists := envVars[k]; !exists {
			envVars[k] = nil
		}
	}
	_, err = c.client.SetEnvironmentVariables(ctx, guid, envVars)
	return errors.Wrap(err, errUpdateResource)
}

// Delete managed resource
func (c *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1alpha1.App)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errWrongKind)
	}

	guid := meta.GetExternalName(cr)
	if _, err := uuid.Parse(guid); err != nil {
		return managed.ExternalDelete{}, errors.New(errDeleteResource + ": No valid GUID found for the App")
	}

	cr.SetConditions(xpv1.Deleting())
	err := c.client.Delete(ctx, guid)
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteResource)
	}

	return managed.ExternalDelete{}, nil
}

// Disconnect implements the managed.ExternalClient interface
func (c *external) Disconnect(ctx context.Context) error {
	// No cleanup needed for Cloud Foundry client
	return nil
}

// getDockerCredential extracts the Docker credentials from the secret
func getDockerCredential(ctx context.Context, kube k8s.Client, forProvider v1alpha1.AppParameters) (*app.DockerCredentials, error) {
	// return immediately if the lifecycle is not docker or credentials are not provided
	if forProvider.Lifecycle != "docker" || forProvider.Docker == nil || forProvider.Docker.Credentials == nil {
		return nil, nil
	}

	buf, err := clients.ExtractSecret(ctx, kube, forProvider.Docker.Credentials, ".dockerconfigjson")
	if err != nil {
		return nil, errors.Wrap(err, errSecret)
	}

	// Parse the JSON to a configfile
	configfile := configfile.New("")
	err = configfile.LoadFromReader(bytes.NewReader(buf))
	if err != nil {
		return nil, errors.Wrap(err, errSecret)
	}

	// TODO: support multiple authentication contexts?
	s := &app.DockerCredentials{}
	if configfile.AuthConfigs != nil {
		for _, authConfig := range configfile.AuthConfigs {
			s.Username = authConfig.Username
			s.Password = authConfig.Password
		}
	}

	return s, nil
}

type initializer struct {
	kube k8s.Client
}

type spaceInitializer initializer

// / Initialize implements the Initializer interface
func (c *spaceInitializer) Initialize(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.App)
	if !ok {
		return errors.New(errWrongKind)
	}

	if cr.Spec.ForProvider.SpaceRef != nil || cr.Spec.ForProvider.SpaceSelector != nil {
		return cr.ResolveReferences(ctx, c.kube)
	}

	return space.ResolveByName(ctx, clients.ClientFnBuilder(ctx, c.kube), mg)
}
