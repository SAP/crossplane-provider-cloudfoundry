//go:build !goverter

package app

import (
	"context"
	"time"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/operation"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/google/uuid"
	"k8s.io/utils/ptr"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/job"
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
}

// ManifestClient defines the interface to communicate with Cloud Foundry Manifest resource.
type ManifestClient interface {
	Generate(ctx context.Context, appGUID string) (string, error)
	ApplyManifest(ctx context.Context, spaceGUID string, manifest string) (string, error)
	ManifestDiff(ctx context.Context, spaceGUID string, manifest string) (*resource.ManifestDiff, error)
}

type Client struct {
	AppClient
	PushClient
	job.Job
	servicecredentialbinding.ServiceCredentialBinding
}

// NewAppClient returns a new AppClient.
func NewAppClient(client *client.Client) *Client {
	return &Client{
		AppClient:                client.Applications,
		PushClient:               NewPushClient(client),
		Job:                      client.Jobs,
		ServiceCredentialBinding: servicecredentialbinding.NewClient(client),
	}
}

type DockerCredentials resource.DockerCredentials

// GetByIDOrSpec gets the App by GUID or spec.
func (c *Client) GetByIDOrSpec(ctx context.Context, guid string, spec v1alpha1.AppParameters) (*resource.App, error) {
	_, err := uuid.Parse(guid)
	if err == nil {
		return c.AppClient.Get(ctx, guid)
	}

	return c.AppClient.Single(ctx, newListOption(spec))
}

// CreateAndPush creates and pushes an app to the Cloud Foundry.
func (c *Client) CreateAndPush(ctx context.Context, spec v1alpha1.AppParameters, dockerCredentials *DockerCredentials) (*resource.App, error) {
	manifest, err := newManifestFromSpec(spec, dockerCredentials)
	if err != nil {
		return nil, err
	}

	application, err := c.AppClient.Create(ctx, newCreateOption(spec))
	if err != nil {
		return nil, err
	}
	return c.PushClient.Push(ctx, application, manifest, nil)
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
	return c.PushClient.Push(ctx, application, manifest, nil)
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

// IsUpToDate checks whether current state is up-to-date compared to the given
// set of parameters.
func IsUpToDate(spec v1alpha1.AppParameters, status v1alpha1.AppObservation) bool {
	if spec.Lifecycle == "docker" && spec.Docker != nil {
		// For docker apps, check if the image has changed
		// The AppManifest in status.AtProvider contains the current image
		// We need to parse the manifest to get the current image
		appManifest, err := getAppManifest(spec.Name, status.AppManifest)
		if err == nil && appManifest.Docker != nil {
			if spec.Docker.Image != appManifest.Docker.Image {
				return false
			}
		}
	}

	// Check for name changes
	if spec.Name != status.Name {
		return false
	}

	// TODO: Add more comprehensive checks for other fields if needed
	return true
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
func newCreateOption(spec v1alpha1.AppParameters) *resource.AppCreate {
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
