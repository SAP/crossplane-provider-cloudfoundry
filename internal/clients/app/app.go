//go:build !goverter

package app

import (
	"context"
	"reflect"
	"time"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/operation"
	"github.com/cloudfoundry/go-cfclient/v3/resource"

	xpresource "github.com/crossplane/crossplane-runtime/pkg/resource"
	"k8s.io/utils/ptr"
	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/job"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/metadata"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/servicecredentialbinding"
)

// AppClient defines the interface to communicate with Cloud Foundry App resource.
type AppClient interface {
	Get(ctx context.Context, guid string) (*resource.App, error)
	Single(ctx context.Context, opts *client.AppListOptions) (*resource.App, error)
	Create(ctx context.Context, r *resource.AppCreate) (*resource.App, error)
	Update(ctx context.Context, guid string, r *resource.AppUpdate) (*resource.App, error)
	Delete(ctx context.Context, guid string) (string, error)

	Start(ctx context.Context, guid string) (*resource.App, error)
	Stop(ctx context.Context, guid string) (*resource.App, error)
	// Restart(ctx context.Context, guid string) (*resource.App, error)
	GetEnvironmentVariables(ctx context.Context, guid string) (map[string]*string, error)
	SetEnvironmentVariables(ctx context.Context, guid string, envVars map[string]*string) (map[string]*string, error)
}

// ManifestClient defines the interface to communicate with Cloud Foundry Manifest resource.
type ManifestClient interface {
	Generate(ctx context.Context, appGUID string) (string, error)
	ApplyManifest(ctx context.Context, spaceGUID string, manifest string) (string, error)
	ManifestDiff(ctx context.Context, spaceGUID string, manifest string) (*resource.ManifestDiff, error)
}

// RouteFetcher defines the interface to fetch routes for an application.
type RouteFetcher interface {
	ListForAppAll(ctx context.Context, appGUID string, opts *client.RouteListOptions) ([]*resource.Route, error)
}

type Client struct {
	AppClient
	PushClient
	job.Job
	servicecredentialbinding.ServiceCredentialBinding
	RouteFetcher
}

// NewAppClient returns a new AppClient.
func NewAppClient(client *client.Client) *Client {
	return &Client{
		AppClient:                client.Applications,
		PushClient:               NewPushClient(client),
		Job:                      client.Jobs,
		ServiceCredentialBinding: servicecredentialbinding.NewClient(client),
		RouteFetcher:             client.Routes,
	}
}

type DockerCredentials resource.DockerCredentials

// GetBySpec gets an App by matching spec fields (name and space).
func (c *Client) GetBySpec(ctx context.Context, spec v1alpha1.AppParameters) (*resource.App, error) {
	return c.AppClient.Single(ctx, newListOption(spec))
}

// CreateAndPush creates and pushes an app to the Cloud Foundry.
func (c *Client) CreateAndPush(ctx context.Context, mg xpresource.Managed, spec v1alpha1.AppParameters, dockerCredentials *DockerCredentials) (*resource.App, error) {
	manifest, err := newManifestFromSpec(spec, dockerCredentials)
	if err != nil {
		return nil, err
	}

	application, err := c.AppClient.Create(ctx, newCreateOption(mg, spec))
	if err != nil {
		return nil, err
	}
	return c.Push(ctx, application, manifest, nil)
}

// Update updates an app in the Cloud Foundry.
func (c *Client) Update(ctx context.Context, guid string, spec v1alpha1.AppParameters) (*resource.App, error) {
	application, err := c.AppClient.Update(ctx, guid, newUpdateOption(spec))
	if err != nil {
		return nil, err
	}
	return application, nil
}

// UpdateAndPush updates and pushes an app to the Cloud Foundry.
func (c *Client) UpdateAndPush(ctx context.Context, guid string, spec v1alpha1.AppParameters, dockerCredentials *DockerCredentials) (*resource.App, error) {
	manifest, err := newManifestFromSpec(spec, dockerCredentials)
	if err != nil {
		return nil, err
	}

	application, err := c.AppClient.Update(ctx, guid, newUpdateOption(spec))
	if err != nil {
		return nil, err
	}
	return c.Push(ctx, application, manifest, nil)
}

// Delete deletes an app in the Cloud Foundry.
func (c *Client) Delete(ctx context.Context, guid string) error {
	jobGUID, err := c.AppClient.Delete(ctx, guid)

	if err != nil {
		return err
	}
	return job.PollJobComplete(ctx, c.Job, jobGUID)
}

// ReconcileServiceBinding updates an app in the Cloud Foundry.
func (c *Client) ReconcileServiceBinding(ctx context.Context, guid string, spec v1alpha1.AppParameters, ymlManifest string) error {

	for _, s := range DiffServiceBindings(spec, ymlManifest) {
		if err := bindService(ctx, c.ServiceCredentialBinding, s); err != nil {
			return err
		}
	}
	return nil
}

// GenerateObservation takes an App resource and returns *AppObservation.
func GenerateObservation(res *resource.App) v1alpha1.AppObservation {
	obs := v1alpha1.AppObservation{}

	obs.GUID = res.GUID
	obs.Name = res.Name
	obs.State = res.State
	obs.CreatedAt = ptr.To(res.CreatedAt.Format(time.RFC3339))
	obs.UpdatedAt = ptr.To(res.UpdatedAt.Format(time.RFC3339))

	return obs
}

// FetchRoutes fetches all routes mapped to the given application and converts
// them to AppRouteObservation values. Errors from the CF API are returned
// non-nil so the caller can decide whether to make them fatal.
// If no RouteFetcher is configured, FetchRoutes returns an empty slice.
func (c *Client) FetchRoutes(ctx context.Context, appGUID string) ([]v1alpha1.AppRouteObservation, error) {
	if c.RouteFetcher == nil {
		return nil, nil
	}
	routes, err := c.ListForAppAll(ctx, appGUID, nil)
	if err != nil {
		return nil, err
	}

	obs := make([]v1alpha1.AppRouteObservation, 0, len(routes))
	for _, r := range routes {
		obs = append(obs, v1alpha1.AppRouteObservation{
			URL:      r.URL,
			Host:     r.Host,
			Path:     r.Path,
			Protocol: r.Protocol,
			Port:     r.Port,
		})
	}
	return obs, nil
}

// ChangeDetection represents what fields have changed
type ChangeDetection struct {
	ChangedFields map[string]struct{}
}

func (cd *ChangeDetection) HasChanges() bool {
	return len(cd.ChangedFields) > 0
}

// HasField checks if a specific field changed
func (cd *ChangeDetection) HasField(field string) bool {
	_, ok := cd.ChangedFields[field]
	return ok
}

// HasOtherChanges returns true if there are changed fields other than the excluded ones.
func (cd *ChangeDetection) HasOtherChanges(excluded ...string) bool {
	excludeSet := make(map[string]struct{}, len(excluded))
	for _, f := range excluded {
		excludeSet[f] = struct{}{}
	}
	for f := range cd.ChangedFields {
		if _, skip := excludeSet[f]; !skip {
			return true
		}
	}
	return false
}

// envVarsChanged returns true if the spec environment variables differ from the current manifest.
func envVarsChanged(spec v1alpha1.AppParameters, appManifest *operation.AppManifest) bool {
	specEnv := spec.Environment
	if specEnv == nil {
		specEnv = map[string]string{}
	}
	currentEnv := appManifest.Env
	if currentEnv == nil {
		currentEnv = map[string]string{}
	}
	return !reflect.DeepEqual(specEnv, currentEnv)
}

// DetectChanges determines what fields have changed between spec and status
func DetectChanges(spec v1alpha1.AppParameters, status v1alpha1.AppObservation) (*ChangeDetection, error) {
	changes := &ChangeDetection{
		ChangedFields: make(map[string]struct{}),
	}

	// Parse manifest once for all checks (treat missing manifest as empty)
	appManifest := &operation.AppManifest{}
	if status.AppManifest != "" {
		m, err := getAppManifest(status.Name, status.AppManifest)
		if err != nil {
			return nil, err
		}
		appManifest = m
	}

	// Check if Docker image changed
	if spec.Lifecycle == "docker" && spec.Docker != nil {
		if appManifest.Docker == nil || spec.Docker.Image != appManifest.Docker.Image {
			changes.ChangedFields["docker_image"] = struct{}{}
		}
	}

	// Check if environment variables changed
	if envVarsChanged(spec, appManifest) {
		changes.ChangedFields["environment"] = struct{}{}
	}

	// Check if name changed
	if spec.Name != status.Name {
		changes.ChangedFields["name"] = struct{}{}
	}

	return changes, nil
}

func IsUpToDate(spec v1alpha1.AppParameters, status v1alpha1.AppObservation) (bool, error) {
	changes, err := DetectChanges(spec, status)
	if err != nil {
		return false, err
	}
	return !changes.HasChanges(), nil
}

// DiffServiceBindings checks whether current state is up-to-date compared to the given
func DiffServiceBindings(spec v1alpha1.AppParameters, ymlManifest string) []v1alpha1.ServiceBindingConfiguration {
	if len(spec.Services) == 0 {
		return nil
	}

	appManifest, err := getAppManifest(spec.Name, ymlManifest)
	if err != nil {
		return nil
	}
	services := make(map[string]operation.AppManifestService)
	if appManifest.Services != nil {
		for _, service := range *appManifest.Services {
			services[service.Name] = service
		}
	}

	var missingServices []v1alpha1.ServiceBindingConfiguration
	for _, service := range spec.Services {
		if _, ok := services[ptr.Deref(service.Name, "")]; !ok {
			return append(missingServices, service)
		}
	}

	return missingServices
}

// newListOption maps spec to AppListOptions
func newListOption(spec v1alpha1.AppParameters) *client.AppListOptions {
	opts := &client.AppListOptions{
		ListOptions: nil,
	}

	opts.Names = client.Filter{Values: []string{spec.Name}}

	if spec.Space != nil {
		opts.SpaceGUIDs = client.Filter{Values: []string{*spec.Space}}
	}

	return opts
}

// newCreateOption maps spec to AppCreate option
func newCreateOption(mg xpresource.Managed, spec v1alpha1.AppParameters) *resource.AppCreate {
	name := spec.Name
	space := ptr.Deref(spec.Space, "")
	appCreate := resource.NewAppCreate(name, space)
	switch spec.Lifecycle {
	case "buildpack":
		appCreate.Lifecycle = &resource.Lifecycle{
			Type: spec.Lifecycle,
			BuildpackData: resource.BuildpackLifecycle{
				Buildpacks: spec.Buildpacks,
				Stack:      ptr.Deref(spec.Stack, ""),
			},
		}
	case "docker":
		appCreate.Lifecycle = &resource.Lifecycle{
			Type: spec.Lifecycle,
		}
	default:
		appCreate.Lifecycle = nil
	}
	appCreate.Metadata = metadata.BuildMetadata(mg, spec.Labels, spec.Annotations)
	return appCreate
}

// newUpdateOption map spec to AppCreate option
func newUpdateOption(spec v1alpha1.AppParameters) *resource.AppUpdate {
	var lifecycle *resource.Lifecycle
	switch spec.Lifecycle {
	case "buildpack":
		lifecycle = &resource.Lifecycle{
			Type: spec.Lifecycle,
			BuildpackData: resource.BuildpackLifecycle{
				Buildpacks: spec.Buildpacks,
				Stack:      ptr.Deref(spec.Stack, ""),
			},
		}
	case "docker":
		lifecycle = &resource.Lifecycle{
			Type: spec.Lifecycle,
		}
	default:
		lifecycle = nil
	}

	return &resource.AppUpdate{
		Name:      spec.Name,
		Lifecycle: lifecycle,
		Metadata:  &resource.Metadata{},
	}
}

// newManifestFromSpec creates a manifest from the given spec.
func bindService(ctx context.Context, scbClient servicecredentialbinding.ServiceCredentialBinding, s v1alpha1.ServiceBindingConfiguration) error {
	// TODO: Implement the binding logic
	return nil
}
