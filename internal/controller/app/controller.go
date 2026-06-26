package app

import (
	"bytes"
	"context"

	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/docker/cli/cli/config/configfile"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
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

	options := []managed.ReconcilerOption{
		managed.WithExternalConnector(
			&connector{kube: mgr.GetClient(),
				usage: resource.NewLegacyProviderConfigUsageTracker(mgr.GetClient(), &pcv1beta1.ProviderConfigUsage{}),
			}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
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
	usage resource.LegacyTracker
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

	if err := c.usage.Track(ctx, mg.(resource.LegacyManaged)); err != nil {
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

	lateInitialized, exists, err := c.resolveExternalName(ctx, cr)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	if !exists {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}
	if lateInitialized {
		return managed.ExternalObservation{ResourceExists: true, ResourceLateInitialized: true}, nil
	}

	guid := meta.GetExternalName(cr)

	if !clients.IsValidGUID(guid) {
		return managed.ExternalObservation{}, errors.Errorf("external-name '%s' is not a valid GUID format", guid)
	}

	res, err := c.client.AppClient.Get(ctx, guid)
	if err != nil {
		if clients.ErrorIsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}

		return managed.ExternalObservation{}, errors.Wrap(err, errObserveResource)
	}

	isUpToDate, err := c.updateObservedStatus(ctx, cr, res)
	if err != nil {
		return managed.ExternalObservation{}, err
	}

	return managed.ExternalObservation{
		ResourceExists:          true,
		ResourceUpToDate:        isUpToDate,
		ResourceLateInitialized: lateInitialized,
	}, nil
}

func (c *external) updateObservedStatus(ctx context.Context, cr *v1alpha1.App, res *cfresource.App) (bool, error) {
	// Preserve previously observed routes so they survive a transient
	// failure from the Routes API.
	prevRoutes := cr.Status.AtProvider.Routes

	// Update the status of the resource
	cr.Status.AtProvider = app.GenerateObservation(res)
	appManifest, err := c.client.GenerateManifest(ctx, res.GUID)
	if err != nil {
		return false, errors.Wrap(err, errObserveResource)
	}
	cr.Status.AtProvider.AppManifest = appManifest

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

	return app.IsUpToDate(cr, cr.Spec.ForProvider, cr.Status.AtProvider)
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

	application, err := c.client.CreateAndPush(ctx, cr, cr.Spec.ForProvider, dockerCredentials)
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

	// Observe validates GUID format before marking ResourceUpToDate:false, so
	// Update is only called with a valid GUID.
	if meta.GetExternalName(cr) == "" {
		return managed.ExternalUpdate{}, nil
	}
	guid := meta.GetExternalName(cr)

	changes, err := app.DetectChanges(cr, cr.Spec.ForProvider, cr.Status.AtProvider)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateResource+": Failed to detect changes")
	}

	if err := c.applyAppUpdates(ctx, guid, cr, changes); err != nil {
		return managed.ExternalUpdate{}, err
	}

	return managed.ExternalUpdate{}, nil
}

func (c *external) applyAppUpdates(ctx context.Context, guid string, cr *v1alpha1.App, changes *app.ChangeDetection) error {
	dockerChanged, err := c.updateDockerImageIfChanged(ctx, guid, cr, changes)
	if err != nil {
		return err
	}

	if err := c.updateEnvironmentIfChanged(ctx, guid, cr, changes, dockerChanged); err != nil {
		return err
	}

	if !changes.HasOtherChanges("docker_image", "environment") {
		return nil
	}

	_, err = c.client.Update(ctx, guid, cr, cr.Spec.ForProvider)
	return errors.Wrap(err, errUpdateResource)
}

func (c *external) updateDockerImageIfChanged(ctx context.Context, guid string, cr *v1alpha1.App, changes *app.ChangeDetection) (bool, error) {
	if !changes.HasField("docker_image") {
		return false, nil
	}
	if err := c.updateDockerImage(ctx, guid, cr); err != nil {
		return false, err
	}
	return true, nil
}

func (c *external) updateEnvironmentIfChanged(ctx context.Context, guid string, cr *v1alpha1.App, changes *app.ChangeDetection, dockerChanged bool) error {
	if !changes.HasField("environment") {
		return nil
	}
	return c.updateEnvVars(ctx, guid, cr, dockerChanged)
}

// updateDockerImage pushes a new docker image for the app.
func (c *external) updateDockerImage(ctx context.Context, guid string, cr *v1alpha1.App) error {
	dockerCredentials, err := getDockerCredential(ctx, c.kube, cr.Spec.ForProvider)
	if err != nil {
		return errors.Wrap(err, errSecret)
	}
	_, err = c.client.UpdateAndPush(ctx, guid, cr, cr.Spec.ForProvider, dockerCredentials)
	return errors.Wrap(err, errUpdateResource)
}

// updateEnvVars updates the environment variables of the app via the CF API directly.
// It sets new/updated vars and sends nil for vars that exist in CF but were removed from spec.
// If the app is currently STOPPED, the restart is skipped (env vars take effect on next start).
// If dockerAlsoChanged is true, the restart is also skipped because the docker push already restarted the app.
func (c *external) updateEnvVars(ctx context.Context, guid string, cr *v1alpha1.App, dockerAlsoChanged bool) error {
	// Build desired env vars from spec
	envVars := map[string]*string{}
	for k, v := range cr.Spec.ForProvider.Environment {
		v := v
		envVars[k] = &v
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
	if err != nil {
		return errors.Wrap(err, errUpdateResource)
	}
	// Restart the app so the updated environment takes effect in the running process.
	// Skip if the app is stopped (env vars take effect on next start) or if
	// docker was also updated (the push already restarted the app).
	if cr.Status.AtProvider.State == "STOPPED" || dockerAlsoChanged {
		return nil
	}
	if _, err = c.client.Stop(ctx, guid); err != nil {
		return errors.Wrap(err, errUpdateResource)
	}
	_, err = c.client.Start(ctx, guid)
	return errors.Wrap(err, errUpdateResource)
}

// Delete managed resource
func (c *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1alpha1.App)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errWrongKind)
	}

	cr.SetConditions(xpv1.Deleting())

	if meta.GetExternalName(cr) == "" {
		return managed.ExternalDelete{}, nil
	}

	err := c.client.Delete(ctx, meta.GetExternalName(cr))
	if err != nil {
		if clients.ErrorIsNotFound(err) {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteResource)
	}

	return managed.ExternalDelete{}, nil
}

// Disconnect implements the managed.ExternalClient interface
func (c *external) Disconnect(ctx context.Context) error {
	// No cleanup needed for Cloud Foundry client
	return nil
}

// resolveExternalName sets the external-name on the App CR if it is empty,
// by looking up the app by spec (name and space).
// Returns (lateInitialized, exists, error).
func (c *external) resolveExternalName(ctx context.Context, cr *v1alpha1.App) (bool, bool, error) {
	if meta.GetExternalName(cr) != "" {
		return false, true, nil
	}

	res, err := c.client.GetBySpec(ctx, cr.Spec.ForProvider)
	if err != nil {
		if clients.ErrorIsNotFound(err) {
			return false, false, nil
		}
		return false, false, errors.Wrap(err, errObserveResource)
	}

	meta.SetExternalName(cr, res.GUID)
	return true, true, nil
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
