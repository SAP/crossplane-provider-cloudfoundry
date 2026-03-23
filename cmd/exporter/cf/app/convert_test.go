/*
Copyright 2025 SAP SE.
*/

package app

import (
	"testing"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"

	"github.com/SAP/xp-clifford/yaml"
	"github.com/cloudfoundry/go-cfclient/v3/operation"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	cpresource "github.com/crossplane/crossplane-runtime/pkg/resource"
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

// mockEventHandler is a test double for export.EventHandler
type mockEventHandler struct {
	warnings  []error
	resources []cpresource.Object
}

func (m *mockEventHandler) Warn(err error) {
	m.warnings = append(m.warnings, err)
}

func (m *mockEventHandler) Resource(r cpresource.Object) {
	m.resources = append(m.resources, r)
}

func (m *mockEventHandler) Stop() {}

// createTestApp creates a test app resource for unit tests
func createTestApp(name, guid, spaceGUID, lifecycle string) *res {
	return &res{
		App: &resource.App{
			Name: name,
			Resource: resource.Resource{
				GUID: guid,
			},
			Lifecycle: resource.Lifecycle{
				Type: lifecycle,
			},
			Relationships: resource.AppRelationships{
				Space: resource.ToOneRelationship{
					Data: &resource.Relationship{
						GUID: spaceGUID,
					},
				},
			},
		},
		ResourceWithComment: yaml.NewResourceWithComment(nil),
	}
}

func TestConvertDockerField(t *testing.T) {
	type args struct {
		app         *res
		managedApp  *v1alpha1.App
		appManifest *operation.AppManifest
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
		check   func(t *testing.T, managedApp *v1alpha1.App)
	}{
		{
			name: "docker image without credentials",
			args: args{
				app:        createTestApp("test-app", "guid-1", "space-guid", "docker"),
				managedApp: &v1alpha1.App{},
				appManifest: &operation.AppManifest{
					Docker: &operation.AppManifestDocker{
						Image: "nginx:latest",
					},
				},
			},
			wantErr: false,
			check: func(t *testing.T, managedApp *v1alpha1.App) {
				if managedApp.Spec.ForProvider.Docker == nil {
					t.Errorf("expected Docker config to be set")
					return
				}
				if managedApp.Spec.ForProvider.Docker.Image != "nginx:latest" {
					t.Errorf("expected Docker image to be 'nginx:latest', got %s", managedApp.Spec.ForProvider.Docker.Image)
				}
				if managedApp.Spec.ForProvider.Docker.Credentials != nil {
					t.Errorf("expected no credentials when username is empty")
				}
			},
		},
		{
			name: "docker image with credentials",
			args: args{
				app:        createTestApp("test-app", "guid-1", "space-guid", "docker"),
				managedApp: &v1alpha1.App{},
				appManifest: &operation.AppManifest{
					Docker: &operation.AppManifestDocker{
						Image:    "private.registry.com/app:1.0",
						Username: "myuser",
					},
				},
			},
			wantErr: false,
			check: func(t *testing.T, managedApp *v1alpha1.App) {
				if managedApp.Spec.ForProvider.Docker == nil {
					t.Errorf("expected Docker config to be set")
					return
				}
				if managedApp.Spec.ForProvider.Docker.Image != "private.registry.com/app:1.0" {
					t.Errorf("expected Docker image to be 'private.registry.com/app:1.0', got %s", managedApp.Spec.ForProvider.Docker.Image)
				}
				if managedApp.Spec.ForProvider.Docker.Credentials == nil {
					t.Errorf("expected credentials to be set")
					return
				}
				if managedApp.Spec.ForProvider.Docker.Credentials.Name == "" {
					t.Errorf("expected secret name to be set")
				}
			},
		},
		{
			name: "missing docker configuration in manifest",
			args: args{
				app:         createTestApp("test-app", "guid-1", "space-guid", "docker"),
				managedApp:  &v1alpha1.App{},
				appManifest: &operation.AppManifest{},
			},
			wantErr: true,
			check: func(t *testing.T, managedApp *v1alpha1.App) {
				if managedApp.Spec.ForProvider.Docker != nil {
					t.Errorf("expected Docker config to be nil when error occurs")
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockHandler := &mockEventHandler{}
			err := convertDockerField(tt.args.app, tt.args.managedApp, tt.args.appManifest, mockHandler)
			if (err != nil) != tt.wantErr {
				t.Errorf("convertDockerField() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.check != nil {
				tt.check(t, tt.args.managedApp)
			}
		})
	}
}

func TestConvertProcessesField(t *testing.T) {
	type args struct {
		managedApp  *v1alpha1.App
		appManifest *operation.AppManifest
	}
	tests := []struct {
		name  string
		args  args
		check func(t *testing.T, managedApp *v1alpha1.App)
	}{
		{
			name: "nil processes",
			args: args{
				managedApp:  &v1alpha1.App{},
				appManifest: &operation.AppManifest{},
			},
			check: func(t *testing.T, managedApp *v1alpha1.App) {
				if len(managedApp.Spec.ForProvider.Processes) != 0 {
					t.Errorf("expected no processes, got %d", len(managedApp.Spec.ForProvider.Processes))
				}
			},
		},
		{
			name: "empty processes",
			args: args{
				managedApp: &v1alpha1.App{},
				appManifest: &operation.AppManifest{
					Processes: &operation.AppManifestProcesses{},
				},
			},
			check: func(t *testing.T, managedApp *v1alpha1.App) {
				if len(managedApp.Spec.ForProvider.Processes) != 0 {
					t.Errorf("expected no processes, got %d", len(managedApp.Spec.ForProvider.Processes))
				}
			},
		},
		{
			name: "single web process",
			args: args{
				managedApp: &v1alpha1.App{},
				appManifest: &operation.AppManifest{
					Processes: &operation.AppManifestProcesses{
						{
							Type:                         "web",
							Command:                      "node server.js",
							DiskQuota:                    "1G",
							Instances:                    ptr.To[uint](2),
							Memory:                       "512M",
							Timeout:                      60,
							HealthCheckType:              "http",
							HealthCheckHTTPEndpoint:      "/health",
							HealthCheckInterval:          30,
							HealthCheckInvocationTimeout: 5,
						},
					},
				},
			},
			check: func(t *testing.T, managedApp *v1alpha1.App) {
				if len(managedApp.Spec.ForProvider.Processes) != 1 {
					t.Errorf("expected 1 process, got %d", len(managedApp.Spec.ForProvider.Processes))
					return
				}
				p := managedApp.Spec.ForProvider.Processes[0]
				if *p.Type != "web" {
					t.Errorf("expected type 'web', got %s", *p.Type)
				}
				if *p.Command != "node server.js" {
					t.Errorf("expected command 'node server.js', got %s", *p.Command)
				}
				if *p.DiskQuota != "1G" {
					t.Errorf("expected disk quota '1G', got %s", *p.DiskQuota)
				}
				if *p.Instances != 2 {
					t.Errorf("expected 2 instances, got %d", *p.Instances)
				}
				if *p.Memory != "512M" {
					t.Errorf("expected memory '512M', got %s", *p.Memory)
				}
				if *p.Timeout != 60 {
					t.Errorf("expected timeout 60, got %d", *p.Timeout)
				}
				if *p.HealthCheckType != "http" {
					t.Errorf("expected health check type 'http', got %s", *p.HealthCheckType)
				}
				if *p.HealthCheckHTTPEndpoint != "/health" {
					t.Errorf("expected health check endpoint '/health', got %s", *p.HealthCheckHTTPEndpoint)
				}
				if *p.HealthCheckInterval != 30 {
					t.Errorf("expected health check interval 30, got %d", *p.HealthCheckInterval)
				}
				if *p.HealthCheckInvocationTimeout != 5 {
					t.Errorf("expected health check invocation timeout 5, got %d", *p.HealthCheckInvocationTimeout)
				}
			},
		},
		{
			name: "multiple processes",
			args: args{
				managedApp: &v1alpha1.App{},
				appManifest: &operation.AppManifest{
					Processes: &operation.AppManifestProcesses{
						{
							Type:      "web",
							Command:   "node server.js",
							Instances: ptr.To[uint](2),
							Memory:    "512M",
						},
						{
							Type:      "worker",
							Command:   "node worker.js",
							Instances: ptr.To[uint](1),
							Memory:    "256M",
						},
					},
				},
			},
			check: func(t *testing.T, managedApp *v1alpha1.App) {
				if len(managedApp.Spec.ForProvider.Processes) != 2 {
					t.Errorf("expected 2 processes, got %d", len(managedApp.Spec.ForProvider.Processes))
					return
				}
				if *managedApp.Spec.ForProvider.Processes[0].Type != "web" {
					t.Errorf("expected first process type 'web', got %s", *managedApp.Spec.ForProvider.Processes[0].Type)
				}
				if *managedApp.Spec.ForProvider.Processes[1].Type != "worker" {
					t.Errorf("expected second process type 'worker', got %s", *managedApp.Spec.ForProvider.Processes[1].Type)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			convertProcessesField(tt.args.managedApp, tt.args.appManifest)
			if tt.check != nil {
				tt.check(t, tt.args.managedApp)
			}
		})
	}
}

func TestGenerateDockerCredentialSecret(t *testing.T) {
	tests := []struct {
		name       string
		secretName string
		username   string
		check      func(t *testing.T, secret *yaml.ResourceWithComment)
	}{
		{
			name:       "basic secret generation",
			secretName: "myapp-docker-credentials",
			username:   "dockeruser",
			check: func(t *testing.T, secretWithComment *yaml.ResourceWithComment) {
				secret, ok := secretWithComment.Resource().(*v1.Secret)
				if !ok {
					t.Fatalf("expected *v1.Secret, got %T", secretWithComment.Resource())
				}
				if secret.Name != "myapp-docker-credentials" {
					t.Errorf("expected name 'myapp-docker-credentials', got %s", secret.Name)
				}
				if secret.Type != v1.SecretTypeOpaque {
					t.Errorf("expected type Opaque, got %s", secret.Type)
				}
				if secret.StringData["username"] != "dockeruser" {
					t.Errorf("expected username 'dockeruser', got %s", secret.StringData["username"])
				}
				if secret.StringData["password"] != "TODO" {
					t.Errorf("expected password placeholder 'TODO', got %s", secret.StringData["password"])
				}
				// Check comment was added
				if comment, _ := secretWithComment.Comment(); comment == "" {
					t.Errorf("expected comment to be added to secret")
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secret := generateDockerCredentialSecret(tt.secretName, tt.username)
			if tt.check != nil {
				tt.check(t, secret)
			}
		})
	}
}

// Note: convertAppResource is tested through integration-style tests
// since it calls getAppManifest which makes external API calls.
// The helper functions (convertDockerField, convertProcessesField) are
// unit tested above.
