package app

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	yaml3 "gopkg.in/yaml.v3"

	cfv3 "github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/operation"
	"github.com/cloudfoundry/go-cfclient/v3/resource"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
)

// PushClient is the interface for pushing an app to the Cloud Foundry
type PushClient interface {
	Push(ctx context.Context, application *resource.App, manifest *operation.AppManifest, dockerCredentials *DockerCredentials) (*resource.App, error)
	GenerateManifest(ctx context.Context, appGUID string) (string, error)
}

// pushClient implements PushClient
type pushClient struct {
	client *cfv3.Client
	// pollOptions for the manifest job and the build; nil uses the go-cfclient defaults
	pollOptions *cfv3.PollingOptions
}

// NewPushClient creates a new PushClient
func NewPushClient(client *cfv3.Client) *pushClient {
	return &pushClient{
		client: client,
	}
}

// Push pushes a docker App to the Cloud Foundry.
//
// It follows the steps of go-cfclient's operation.AppPushOperation, but passes
// the docker registry password directly in the package request. The go-cfclient
// operation reads it from the process-wide CF_DOCKER_PASSWORD environment
// variable, which is shared by all concurrent reconciles.
func (p *pushClient) Push(ctx context.Context, application *resource.App, manifest *operation.AppManifest, dockerCredentials *DockerCredentials) (*resource.App, error) {
	if manifest.Docker == nil {
		return nil, errors.New("docker configuration is required to push an app")
	}
	spaceGUID := application.Relationships.Space.Data.GUID
	if err := p.applyManifest(ctx, spaceGUID, manifest); err != nil {
		return nil, err
	}

	app, err := p.client.Applications.Get(ctx, application.GUID)
	if err != nil {
		return nil, err
	}
	appWasStarted := app.State == "STARTED"

	pkg, err := p.createDockerPackage(ctx, app.GUID, manifest.Docker, dockerCredentials)
	if err != nil {
		return nil, err
	}

	droplet, err := p.buildDroplet(ctx, pkg.GUID, manifest.Name)
	if err != nil {
		return nil, err
	}

	if _, err := p.client.Droplets.SetCurrentAssociationForApp(ctx, app.GUID, droplet.GUID); err != nil {
		return nil, errors.Wrapf(err, "error setting current droplet for app %s", manifest.Name)
	}

	if appWasStarted {
		return p.client.Applications.Restart(ctx, app.GUID)
	}
	return p.client.Applications.Start(ctx, app.GUID)
}

// applyManifest applies the app manifest to the space and waits for the job to finish
func (p *pushClient) applyManifest(ctx context.Context, spaceGUID string, manifest *operation.AppManifest) error {
	// wrap it in a manifest that has an applications array as required by the API
	m := &operation.Manifest{Applications: []*operation.AppManifest{manifest}}
	manifestBytes, err := yaml3.Marshal(m)
	if err != nil {
		return errors.Wrap(err, "error marshalling application manifest")
	}

	jobGUID, err := p.client.Manifests.ApplyManifest(ctx, spaceGUID, string(manifestBytes))
	if err != nil {
		return errors.Wrapf(err, "error applying application manifest to space %s", spaceGUID)
	}
	if err := p.client.Jobs.PollComplete(ctx, jobGUID, p.pollOptions); err != nil {
		return errors.Wrapf(err, "error waiting for application manifest to finish applying to space %s", spaceGUID)
	}
	return nil
}

// createDockerPackage creates a docker package with the registry credentials for this app only
func (p *pushClient) createDockerPackage(ctx context.Context, appGUID string, docker *operation.AppManifestDocker, dockerCredentials *DockerCredentials) (*resource.Package, error) {
	newPkg := newDockerPackageCreate(appGUID, docker, dockerCredentials)
	pkg, err := p.client.Packages.Create(ctx, newPkg)
	if err != nil {
		return nil, errors.Wrapf(err, "error creating docker package for app %s", appGUID)
	}
	return pkg, nil
}

// newDockerPackageCreate builds the package request; without credentials none are sent
func newDockerPackageCreate(appGUID string, docker *operation.AppManifestDocker, dockerCredentials *DockerCredentials) *resource.PackageCreate {
	username, password := "", ""
	if dockerCredentials != nil {
		username, password = dockerCredentials.Username, dockerCredentials.Password
	}
	newPkg := resource.NewDockerPackageCreate(appGUID, docker.Image, username, password)
	if username == "" && password == "" {
		newPkg.Data.DockerCredentials = nil
	}
	return newPkg
}

// buildDroplet stages the package and returns the resulting droplet
func (p *pushClient) buildDroplet(ctx context.Context, pkgGUID string, appName string) (*resource.Droplet, error) {
	newBuild := resource.NewBuildCreate(pkgGUID)
	newBuild.Lifecycle = &resource.Lifecycle{
		Type: resource.LifecycleDocker.String(),
		Data: &resource.DockerLifecycle{},
	}
	build, err := p.client.Builds.Create(ctx, newBuild)
	if err != nil {
		return nil, errors.Wrapf(err, "error creating build from package for app %s", appName)
	}
	if err := p.client.Builds.PollStaged(ctx, build.GUID, p.pollOptions); err != nil {
		return nil, errors.Wrapf(err, "error while waiting for app %s package to build", appName)
	}

	opts := cfv3.NewDropletPackageListOptions()
	opts.States.EqualTo(resource.DropletStateStaged.String())
	droplet, err := p.client.Droplets.SingleForPackage(ctx, pkgGUID, opts)
	if err != nil {
		return nil, errors.Wrapf(err, "error finding droplet for app %s", appName)
	}
	return droplet, nil
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

	docker, err := configDocker(forProvider, dockerCredentials)
	if err != nil {
		return nil, err
	}
	manifest.Docker = docker

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

	if len(forProvider.Environment) > 0 {
		manifest.Env = forProvider.Environment
	}

	return manifest, nil
}

func configDocker(forProvider v1alpha1.AppParameters, dockerCredentials *DockerCredentials) (*operation.AppManifestDocker, error) {

	if forProvider.Lifecycle != "" && forProvider.Lifecycle != "docker" {
		return nil, errors.Errorf("unsupported lifecycle %q: only docker is supported", forProvider.Lifecycle)
	}
	if forProvider.Docker == nil || forProvider.Docker.Image == "" {
		return nil, errors.New("docker image is required: set spec.forProvider.docker.image, only the docker lifecycle is supported")
	}
	docker := &operation.AppManifestDocker{
		Image: forProvider.Docker.Image,
	}

	// The password is not part of the manifest; Push sends it with the docker package.
	if dockerCredentials != nil {
		docker.Username = dockerCredentials.Username
	}

	return docker, nil
}

// configProcess map the process from app spec
//
//nolint:gocyclo
func configProcess(forProvider v1alpha1.AppParameters) *operation.AppManifestProcesses {
	if len(forProvider.Processes) > 0 {
		processes := make(operation.AppManifestProcesses, 0, len(forProvider.Processes))
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
