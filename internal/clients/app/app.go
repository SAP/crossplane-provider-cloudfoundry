package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/operation"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/google/uuid"
	"k8s.io/utils/ptr"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha2"
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
}

// NewAppClient returns a new AppClient.
func NewAppClient(client *client.Client) *Client {
	return &Client{
		AppClient:  client.Applications,
		PushClient: NewPushClient(client),
	}
}

// GetByIDOrSpec gets the App by GUID or spec.
func (c *Client) GetByIDOrSpec(ctx context.Context, guid string, spec v1alpha2.AppParameters) (*resource.App, error) {
	_, err := uuid.Parse(guid)
	if err == nil {
		return c.AppClient.Get(ctx, guid)
	}

	return c.AppClient.Single(ctx, newListOption(spec))
}

// CreateAndPush creates and pushes an app to the Cloud Foundry.
func (c *Client) CreateAndPush(ctx context.Context, spec v1alpha2.AppParameters, dockerCredentialExtractor DockerCredentialExtractor) (*resource.App, error) {
	manifest := newManifestFromSpec(spec)
	switch spec.Lifecycle {
	case "docker":
		if spec.Docker == nil {
			return nil, errors.New("docker lifecycle requires docker spec")
		}
		manifest.Docker = &operation.AppManifestDocker{
			Image:    spec.Docker.Image,
			Username: "",
		}
		if spec.Docker.Credentials != nil {
			credentials, err := dockerCredentialExtractor(*spec.Docker.Credentials)
			if err == nil {
				manifest.Docker.Username = credentials.Username
				err = os.Setenv("CF_DOCKER_PASSWORD", credentials.Password)
				if err != nil {
					return nil, err
				}
			}
		}
	case "buildpack":
		return nil, errors.New("buildpack lifecycle is not supported")
	default:
		return nil, fmt.Errorf("unknown lifecycle: %s", spec.Lifecycle)
	}

	application, err := c.AppClient.Create(ctx, newCreateOption(spec))
	if err != nil {
		return nil, err
	}
	return c.PushClient.Push(ctx, application, manifest, nil)
}

// Update updates an app in the Cloud Foundry.
func (c *Client) Update(ctx context.Context, guid string, spec v1alpha2.AppParameters, dockerCredentialExtractor DockerCredentialExtractor) (*resource.App, error) {
	application, err := c.AppClient.Update(ctx, guid, newUpdateOption(spec))
	if err != nil {
		return nil, err
	}

	//TODO: We need to check where app manifest is change and push is required.

	return application, nil

}

// GenerateObservation takes an App resource and returns *AppObservation.
func GenerateObservation(res *resource.App) v1alpha2.AppObservation {
	obs := v1alpha2.AppObservation{}

	obs.ID = res.GUID
	obs.Name = res.Name
	obs.State = res.State
	obs.CreatedAt = ptr.To(res.CreatedAt.Format(time.RFC3339))
	obs.UpdatedAt = ptr.To(res.UpdatedAt.Format(time.RFC3339))

	return obs
}

// LateInitialize fills the unassigned fields with values from a App resource.
func LateInitialize(spec *v1alpha2.AppParameters, res *resource.App) {
	// Do nothing yet
}

// IsUpToDate checks whether current state is up-to-date compared to the given
// set of parameters.
func IsUpToDate(spec v1alpha2.AppParameters, res *resource.App) bool {
	// rename or update ssh setting
	return spec.Name == res.Name

}

// newListOption maps spec to AppListOptions
func newListOption(spec v1alpha2.AppParameters) *client.AppListOptions {
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
func newCreateOption(spec v1alpha2.AppParameters) *resource.AppCreate {
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
func newUpdateOption(spec v1alpha2.AppParameters) *resource.AppUpdate {
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
