package app

import (
	"context"
	"encoding/json"
	"io"
	"os"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"

	cfv3 "github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/operation"
	"github.com/cloudfoundry/go-cfclient/v3/resource"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
)

// PushClient is the interface for pushing an app to the Cloud Foundry
type PushClient interface {
	Push(ctx context.Context, application *resource.App, manifest *operation.AppManifest, zipFile io.Reader) (*resource.App, error)
	GenerateManifest(ctx context.Context, appGUID string) (string, error)
}

// pushClient implements PushClient
type pushClient struct {
	client *cfv3.Client
}

// NewPushClient creates a new PushClient
func NewPushClient(client *cfv3.Client) *pushClient {
	return &pushClient{
		client: client,
	}
}

// newAppPushOperation initializes a push operation with app target
func newAppPushOperation(ctx context.Context, client *cfv3.Client, application *resource.App) (*operation.AppPushOperation, error) {

	space, err := client.Spaces.Get(ctx, application.Relationships.Space.Data.GUID)
	if err != nil {
		return nil, err
	}
	org, err := client.Organizations.Get(ctx, space.Relationships.Organization.Data.GUID)
	if err != nil {
		return nil, err
	}

	return operation.NewAppPushOperation(client, org.Name, space.Name), nil
}

// Push pushes an App to the Cloud Foundry
func (p *pushClient) Push(ctx context.Context, application *resource.App, manifest *operation.AppManifest, zipfile io.Reader) (*resource.App, error) {
	pusher, err := newAppPushOperation(ctx, p.client, application)
	if err != nil {
		return nil, err
	}
	return pusher.Push(ctx, manifest, nil)
}

// GenerateManifest generates a manifest for the app
func (p *pushClient) GenerateManifest(ctx context.Context, appGUID string) (string, error) {
	return p.client.Manifests.Generate(ctx, appGUID)
}

// newManifest maps the app spec to the manifest
//
//nolint:gocyclo
func newManifestFromSpec(forProvider v1alpha1.AppParameters, dockerCredentials *DockerCredentials) (*operation.AppManifest, error) {
	manifest := operation.NewAppManifest(forProvider.Name)

	if forProvider.Lifecycle == "docker" {
		docker, err := configDocker(forProvider, dockerCredentials)
		if err != nil {
			return nil, err
		}
		manifest.Docker = docker
	}

	services, err := configServices(forProvider)
	if err != nil {
		return nil, err
	}
	manifest.Services = services

	if forProvider.NoRoute {
		manifest.NoRoute = true
	}
	if forProvider.RandomRoute {
		manifest.RandomRoute = true
	}
	if forProvider.DefaultRoute {
		manifest.DefaultRoute = true
	}
	manifest.Routes = configRoutes(forProvider)

	manifest.Processes = configProcess(forProvider)

	if forProvider.ReadinessHealthCheckType != nil {
		manifest.ReadinessHealthCheckType = *forProvider.ReadinessHealthCheckType
	}

	if forProvider.ReadinessHealthCheckHTTPEndpoint != nil {
		manifest.ReadinessHealthCheckHttpEndpoint = *forProvider.ReadinessHealthCheckHTTPEndpoint
	}

	if forProvider.ReadinessHealthCheckInterval != nil {
		manifest.ReadinessHealthCheckInterval = *forProvider.ReadinessHealthCheckInterval
	}

	if forProvider.ReadinessHealthCheckInvocationTimeout != nil {
		manifest.ReadinessHealthInvocationTimeout = *forProvider.ReadinessHealthCheckInvocationTimeout
	}

	if forProvider.LogRateLimitPerSecond != nil {
		manifest.LogRateLimitPerSecond = *forProvider.LogRateLimitPerSecond
	}
	return manifest, nil
}

func configDocker(forProvider v1alpha1.AppParameters, dockerCredentials *DockerCredentials) (*operation.AppManifestDocker, error) {

	if forProvider.Docker == nil {
		return nil, errors.New("docker lifecycle requires docker spec")
	}
	docker := &operation.AppManifestDocker{
		Image: forProvider.Docker.Image,
	}

	if dockerCredentials != nil {
		docker.Username = dockerCredentials.Username
		err := os.Setenv("CF_DOCKER_PASSWORD", dockerCredentials.Password)
		if err != nil {
			return nil, err
		}
	}

	return docker, nil
}

// configProcess map the process from app spec
//
//nolint:gocyclo
func configProcess(forProvider v1alpha1.AppParameters) *operation.AppManifestProcesses {
	if len(forProvider.Processes) > 0 {
		var processes operation.AppManifestProcesses
		for _, process := range forProvider.Processes {
			processManifest := operation.AppManifestProcess{}
			if process.Type != nil {
				processManifest.Type = operation.AppProcessType(*process.Type)
			}
			if process.Command != nil {
				processManifest.Command = *process.Command
			}
			if process.HealthCheckType != nil {
				processManifest.HealthCheckType = operation.AppHealthCheckType(*process.HealthCheckType)
			}
			if process.HealthCheckHTTPEndpoint != nil {
				processManifest.HealthCheckHTTPEndpoint = *process.HealthCheckHTTPEndpoint
			}
			if process.HealthCheckInvocationTimeout != nil {
				processManifest.HealthCheckInvocationTimeout = *process.HealthCheckInvocationTimeout
			}
			if process.HealthCheckInterval != nil {
				processManifest.HealthCheckInterval = *process.HealthCheckInterval
			}
			if process.DiskQuota != nil {
				processManifest.DiskQuota = *process.DiskQuota
			}
			if process.Memory != nil {
				processManifest.Memory = *process.Memory
			}
			if process.Timeout != nil {
				processManifest.Timeout = *process.Timeout
			}
			if process.Instances != nil {
				processManifest.Instances = process.Instances
			}

			processes = append(processes, processManifest)
		}
		return &processes
	}
	return nil

}

// configServices map the services from app spec
func configServices(forProvider v1alpha1.AppParameters) (*operation.AppManifestServices, error) {
	if len(forProvider.Services) > 0 {
		var services operation.AppManifestServices
		for _, service := range forProvider.Services {
			if service.Name != nil {
				m := operation.AppManifestService{
					Name: *service.Name,
				}

				if service.BindingName != "" {
					m.BindingName = service.BindingName
				}
				if service.Parameters.Raw != nil {
					// Convert to map[string]interface{}
					var params map[string]interface{}
					if err := json.Unmarshal(service.Parameters.Raw, &params); err != nil {
						return nil, errors.Wrap(err, "failed to unmarshal service parameters")
					}
					m.Parameters = params
				}
				services = append(services, m)
			}
		}
		return &services, nil
	}
	return nil, nil
}

// configRoutes map the routes from app spec
func configRoutes(forProvider v1alpha1.AppParameters) *operation.AppManifestRoutes {
	if len(forProvider.Routes) > 0 {
		var routes operation.AppManifestRoutes
		for _, route := range forProvider.Routes {
			if route.Route != nil {
				routes = append(routes, operation.AppManifestRoute{
					Route: *route.Route,
				})
			}
		}
		return &routes
	}
	return nil
}

// getAppManifest returns the app manifest from the manifest file
func getAppManifest(appName string, strManifest string) (*operation.AppManifest, error) {

	m := operation.Manifest{}
	err := yaml.Unmarshal([]byte(strManifest), &m)
	if err != nil {
		return nil, err
	}
	for _, app := range m.Applications {
		if app.Name == appName {
			return app, nil
		}
	}
	return nil, errors.New("app not found in manifest")
}
