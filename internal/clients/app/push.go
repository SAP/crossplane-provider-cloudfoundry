package app

import (
	"context"
	"io"

	cfv3 "github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/operation"
	"github.com/cloudfoundry/go-cfclient/v3/resource"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha2"
)

// PushClient is the interface for pushing an app to the Cloud Foundry
type PushClient interface {
	Push(ctx context.Context, application *resource.App, manifest *operation.AppManifest, zipFile io.Reader) (*resource.App, error)
	GenerateManifest(ctx context.Context, appGUID string) (string, error)
}

// DockerCredentials represents the docker credentials
type DockerCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// DockerCredentialExtractor is a function that extracts the docker credentials from the secret
type DockerCredentialExtractor func(credentials v1alpha2.DockerCredentials) (*DockerCredentials, error)

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
func newManifestFromSpec(appSpec v1alpha2.AppParameters) *operation.AppManifest {
	manifest := operation.NewAppManifest(appSpec.Name)
	configRoutes(manifest, appSpec)
	configServices(manifest, appSpec)
	configProcess(manifest, appSpec)
	configReadinessCheck(manifest, appSpec)
	if appSpec.LogRateLimitPerSecond != nil {
		manifest.LogRateLimitPerSecond = *appSpec.LogRateLimitPerSecond
	}
	return manifest
}

// configProcess map the process from app spec
func configProcess(manifest *operation.AppManifest, appSpec v1alpha2.AppParameters) {
	if manifest == nil {
		return
	}

	if len(appSpec.Processes) > 0 {
		var processes operation.AppManifestProcesses
		for _, process := range appSpec.Processes {
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

			processes = append(processes, processManifest)
		}
		manifest.Processes = &processes
	}

}

func configReadinessCheck(manifest *operation.AppManifest, appSpec v1alpha2.AppParameters) {
	if manifest == nil {
		return
	}
	if appSpec.ReadinessHealthCheckType != nil {
		manifest.ReadinessHealthCheckType = *appSpec.ReadinessHealthCheckType
	}

	if appSpec.ReadinessHealthCheckHTTPEndpoint != nil {
		manifest.ReadinessHealthCheckHttpEndpoint = *appSpec.ReadinessHealthCheckHTTPEndpoint
	}

	if appSpec.ReadinessHealthCheckInterval != nil {
		manifest.ReadinessHealthCheckInterval = *appSpec.ReadinessHealthCheckInterval
	}

	if appSpec.ReadinessHealthCheckInvocationTimeout != nil {
		manifest.ReadinessHealthInvocationTimeout = *appSpec.ReadinessHealthCheckInvocationTimeout
	}
}

// TODO: This implementation is not complete
// configServices map the services from app spec
func configServices(manifest *operation.AppManifest, appSpec v1alpha2.AppParameters) {
	if manifest == nil {
		return
	}

	if len(appSpec.Services) > 0 {
		var services operation.AppManifestServices
		for _, service := range appSpec.Services {
			if service.Name != nil {
				services = append(services, operation.AppManifestService{
					Name: *service.Name,
				})
			}
		}
		manifest.Services = &services
	}
}

// TODO: This implementation is not complete. Logic of referencing the routes is still missing
// configRoutes map the routes from app spec
func configRoutes(manifest *operation.AppManifest, appSpec v1alpha2.AppParameters) {
	if manifest == nil {
		return
	}

	if appSpec.NoRoute {
		manifest.NoRoute = true
		return
	}

	if len(appSpec.Routes) > 0 {
		var routes operation.AppManifestRoutes
		for _, route := range appSpec.Routes {
			if route.Route != nil {
				routes = append(routes, operation.AppManifestRoute{
					Route: *route.Route,
				})
			}
		}
		manifest.Routes = &routes
		return
	}

	if appSpec.RandomRoute {
		manifest.RandomRoute = true
		return
	}

	manifest.DefaultRoute = true
}
