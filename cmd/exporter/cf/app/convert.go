// Package app implements Cloud Foundry App resource export functionality.
package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/space"

	"github.com/SAP/xp-clifford/cli/export"
	"github.com/SAP/xp-clifford/erratt"
	"github.com/SAP/xp-clifford/parsan"
	"github.com/SAP/xp-clifford/yaml"
	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/operation"
	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// convertDockerField converts Docker configuration from CF manifest to Crossplane DockerConfiguration.
// Generates a Secret for Docker credentials if username is provided. Password is set to "TODO" placeholder.
func convertDockerField(app *res, managedApp *v1alpha1.App, appManifest *operation.AppManifest, evHandler export.EventHandler) error {
	docker := appManifest.Docker

	if docker == nil {
		return erratt.New("invalid application manifest: missing docker",
			"app-name", app.GetName(),
			"app-manifest", appManifest)
	}

	managedApp.Spec.ForProvider.Docker = &v1alpha1.DockerConfiguration{
		Image: docker.Image,
	}
	if username := docker.Username; username != "" {
		secretNames := parsan.ParseAndSanitize(
			fmt.Sprintf("%s.docker-credentials", app.GetName()),
			parsan.RFC1035SubdomainRelaxed)
		if len(secretNames) == 0 {
			erra := erratt.New(
				"cannot sanitize docker credentials secret name",
				"app-name", app.GetName(),
			)
			evHandler.Warn(erra)
		} else {
			secretName := secretNames[0]
			evHandler.Resource(generateDockerCredentialSecret(secretName, username))
			managedApp.Spec.ForProvider.Docker.Credentials = &v1.SecretReference{
				Name: secretName,
			}
		}
	}
	return nil
}

// convertProcessesField converts CF application processes to Crossplane ProcessConfiguration.
// Handles all process types including web and worker processes with health checks.
func convertProcessesField(managedApp *v1alpha1.App, appManifest *operation.AppManifest) {
	if appManifest.Processes == nil {
		return
	}
	if len(*appManifest.Processes) == 0 {
		return
	}

	managedApp.Spec.ForProvider.Processes = make([]v1alpha1.ProcessConfiguration, len(*appManifest.Processes))
	for i, process := range *appManifest.Processes {
		managedApp.Spec.ForProvider.Processes[i] = v1alpha1.ProcessConfiguration{
			Type:      (*string)(&process.Type),
			Command:   &process.Command,
			DiskQuota: &process.DiskQuota,
			Instances: process.Instances,
			Memory:    &process.Memory,
			Timeout:   &process.Timeout,
			HealthCheckConfiguration: v1alpha1.HealthCheckConfiguration{
				HealthCheckType:              (*string)(&process.HealthCheckType),
				HealthCheckHTTPEndpoint:      &process.HealthCheckHTTPEndpoint,
				HealthCheckInterval:          &process.HealthCheckInterval,
				HealthCheckInvocationTimeout: &process.HealthCheckInvocationTimeout,
			},
		}
	}
}

// convertAppResource converts a CF application to a Crossplane App resource with manifest data.
// Fetches the app manifest, converts Docker config and processes, and optionally resolves space references.
// Returns a ResourceWithComment containing the converted App and any warning comments.
func convertAppResource(ctx context.Context, cfClient *client.Client, app *res, evHandler export.EventHandler, resolveReferences bool) *yaml.ResourceWithComment {
	slog.Debug("converting app", "name", app.Name)

	managedApp := &v1alpha1.App{
		TypeMeta: metav1.TypeMeta{
			Kind:       v1alpha1.App_Kind,
			APIVersion: v1alpha1.CRDGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: app.GetName(),
			Annotations: map[string]string{
				"crossplane.io/external-name": app.GetGUID(),
			},
		},
		Spec: v1alpha1.AppSpec{
			ResourceSpec: v1.ResourceSpec{
				ManagementPolicies: []v1.ManagementAction{
					v1.ManagementActionObserve,
				},
			},
			ForProvider: v1alpha1.AppParameters{
				Name:      app.GetName(),
				Lifecycle: app.Lifecycle.Type,
				SpaceReference: v1alpha1.SpaceReference{
					Space: &app.Relationships.Space.Data.GUID,
				},
				// Buildpacks:                        []string{}, // not supported yet
				// Stack:                             new(string), // not supported yet
				// Path:                              new(string), // not supported yet
				// Routes:                            []v1alpha1.RouteConfiguration{}, // not supported yet
				// Services:                          []v1alpha1.ServiceBindingConfiguration{}, // not supported yet

				// Environment:                       &runtime.RawExtension{}, // not supported yetk
			},
		},
	}

	appWithComment := yaml.NewResourceWithComment(managedApp)
	appWithComment.CloneComment(app.ResourceWithComment)

	appManifest, err := getAppManifest(ctx, cfClient, app.GetGUID())
	if err != nil {
		erra := erratt.Errorf("cannot get app manifest: %w", err).With("app-name", app.GetName())
		evHandler.Warn(erra)
		appWithComment.AddComment(erra.Error())
	} else {
		slog.Debug("app manifest fetched", "app-manifest", appManifest, "app-name", app.GetName())

		if app.Lifecycle.Type == "docker" {
			err = convertDockerField(app, managedApp, appManifest, evHandler)
			if err != nil {
				evHandler.Warn(err)
				appWithComment.AddComment(err.Error())
			}
		}
		managedApp.Spec.ForProvider.NoRoute = appManifest.NoRoute
		managedApp.Spec.ForProvider.RandomRoute = appManifest.RandomRoute
		managedApp.Spec.ForProvider.DefaultRoute = appManifest.DefaultRoute
		convertProcessesField(managedApp, appManifest)
		managedApp.Spec.ForProvider.ReadinessHealthCheckConfiguration = v1alpha1.ReadinessHealthCheckConfiguration{
			ReadinessHealthCheckType:              &appManifest.ReadinessHealthCheckType,
			ReadinessHealthCheckHTTPEndpoint:      &appManifest.ReadinessHealthCheckHttpEndpoint,
			ReadinessHealthCheckInterval:          &appManifest.ReadinessHealthCheckInterval,
			ReadinessHealthCheckInvocationTimeout: &appManifest.ReadinessHealthInvocationTimeout,
		}
		managedApp.Spec.ForProvider.LogRateLimitPerSecond = &appManifest.LogRateLimitPerSecond
	}
	if resolveReferences {
		if err := space.ResolveReference(ctx, cfClient, &managedApp.Spec.ForProvider.SpaceReference); err != nil {
			erra := erratt.Errorf("cannot resolve space reference: %w", err).With("app-name", app.GetName())
			evHandler.Warn(erra)
		}
	}
	return appWithComment
}
