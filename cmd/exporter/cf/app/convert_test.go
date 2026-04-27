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
		{
			name:       "empty username",
			secretName: "myapp-docker-credentials",
			username:   "",
			check: func(t *testing.T, secretWithComment *yaml.ResourceWithComment) {
				secret, ok := secretWithComment.Resource().(*v1.Secret)
				if !ok {
					t.Fatalf("expected *v1.Secret, got %T", secretWithComment.Resource())
				}
				if secret.Name != "myapp-docker-credentials" {
					t.Errorf("expected name 'myapp-docker-credentials', got %s", secret.Name)
				}
				if secret.StringData["username"] != "" {
					t.Errorf("expected empty username, got %s", secret.StringData["username"])
				}
				if secret.StringData["password"] != "TODO" {
					t.Errorf("expected password placeholder 'TODO', got %s", secret.StringData["password"])
				}
			},
		},
		{
			name:       "empty secret name",
			secretName: "",
			username:   "dockeruser",
			check: func(t *testing.T, secretWithComment *yaml.ResourceWithComment) {
				secret, ok := secretWithComment.Resource().(*v1.Secret)
				if !ok {
					t.Fatalf("expected *v1.Secret, got %T", secretWithComment.Resource())
				}
				if secret.Name != "" {
					t.Errorf("expected empty secret name, got %s", secret.Name)
				}
				if secret.StringData["username"] != "dockeruser" {
					t.Errorf("expected username 'dockeruser', got %s", secret.StringData["username"])
				}
			},
		},
		{
			name:       "both empty",
			secretName: "",
			username:   "",
			check: func(t *testing.T, secretWithComment *yaml.ResourceWithComment) {
				secret, ok := secretWithComment.Resource().(*v1.Secret)
				if !ok {
					t.Fatalf("expected *v1.Secret, got %T", secretWithComment.Resource())
				}
				if secret.Name != "" {
					t.Errorf("expected empty secret name, got %s", secret.Name)
				}
				if secret.StringData["username"] != "" {
					t.Errorf("expected empty username, got %s", secret.StringData["username"])
				}
				if secret.StringData["password"] != "TODO" {
					t.Errorf("expected password placeholder 'TODO', got %s", secret.StringData["password"])
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

func TestConvertProcessConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		process operation.AppManifestProcess
		check   func(t *testing.T, cfg *v1alpha1.ProcessConfiguration)
	}{
		{
			name: "process with all fields set",
			process: operation.AppManifestProcess{
				Type:                         "web",
				Command:                      "node server.js",
				DiskQuota:                    "1G",
				Instances:                    ptr.To[uint](3),
				Memory:                       "512M",
				Timeout:                      60,
				HealthCheckType:              "http",
				HealthCheckHTTPEndpoint:      "/health",
				HealthCheckInterval:          30,
				HealthCheckInvocationTimeout: 5,
			},
			check: func(t *testing.T, cfg *v1alpha1.ProcessConfiguration) {
				if *cfg.Type != "web" {
					t.Errorf("expected type 'web', got %s", *cfg.Type)
				}
				if *cfg.Command != "node server.js" {
					t.Errorf("expected command 'node server.js', got %s", *cfg.Command)
				}
				if *cfg.DiskQuota != "1G" {
					t.Errorf("expected disk quota '1G', got %s", *cfg.DiskQuota)
				}
				if *cfg.Instances != 3 {
					t.Errorf("expected 3 instances, got %d", *cfg.Instances)
				}
				if *cfg.Memory != "512M" {
					t.Errorf("expected memory '512M', got %s", *cfg.Memory)
				}
				if *cfg.Timeout != 60 {
					t.Errorf("expected timeout 60, got %d", *cfg.Timeout)
				}
				if *cfg.HealthCheckType != "http" {
					t.Errorf("expected health check type 'http', got %s", *cfg.HealthCheckType)
				}
				if *cfg.HealthCheckHTTPEndpoint != "/health" {
					t.Errorf("expected health check endpoint '/health', got %s", *cfg.HealthCheckHTTPEndpoint)
				}
				if *cfg.HealthCheckInterval != 30 {
					t.Errorf("expected health check interval 30, got %d", *cfg.HealthCheckInterval)
				}
				if *cfg.HealthCheckInvocationTimeout != 5 {
					t.Errorf("expected health check invocation timeout 5, got %d", *cfg.HealthCheckInvocationTimeout)
				}
			},
		},
		{
			name: "process with only required fields",
			process: operation.AppManifestProcess{
				Type:            "worker",
				Instances:       ptr.To[uint](1),
				HealthCheckType: "port",
			},
			check: func(t *testing.T, cfg *v1alpha1.ProcessConfiguration) {
				if *cfg.Type != "worker" {
					t.Errorf("expected type 'worker', got %s", *cfg.Type)
				}
				if cfg.Command != nil {
					t.Errorf("expected nil command for empty value, got %v", *cfg.Command)
				}
				if cfg.DiskQuota != nil {
					t.Errorf("expected nil disk quota for empty value, got %v", *cfg.DiskQuota)
				}
				if *cfg.Instances != 1 {
					t.Errorf("expected 1 instance, got %d", *cfg.Instances)
				}
				if cfg.Memory != nil {
					t.Errorf("expected nil memory for empty value, got %v", *cfg.Memory)
				}
				if cfg.Timeout != nil {
					t.Errorf("expected nil timeout for zero value, got %v", *cfg.Timeout)
				}
				if *cfg.HealthCheckType != "port" {
					t.Errorf("expected health check type 'port', got %s", *cfg.HealthCheckType)
				}
				if cfg.HealthCheckHTTPEndpoint != nil {
					t.Errorf("expected nil health check endpoint for empty value, got %v", *cfg.HealthCheckHTTPEndpoint)
				}
				if cfg.HealthCheckInterval != nil {
					t.Errorf("expected nil health check interval for zero value, got %v", *cfg.HealthCheckInterval)
				}
				if cfg.HealthCheckInvocationTimeout != nil {
					t.Errorf("expected nil health check invocation timeout for zero value, got %v", *cfg.HealthCheckInvocationTimeout)
				}
			},
		},
		{
			name: "process with partial fields",
			process: operation.AppManifestProcess{
				Type:                "web",
				Command:             "python app.py",
				Instances:           ptr.To[uint](2),
				Memory:              "256M",
				Timeout:             30,
				HealthCheckType:     "http",
				HealthCheckInterval: 10,
			},
			check: func(t *testing.T, cfg *v1alpha1.ProcessConfiguration) {
				if *cfg.Type != "web" {
					t.Errorf("expected type 'web', got %s", *cfg.Type)
				}
				if *cfg.Command != "python app.py" {
					t.Errorf("expected command 'python app.py', got %s", *cfg.Command)
				}
				if cfg.DiskQuota != nil {
					t.Errorf("expected nil disk quota for empty value, got %v", *cfg.DiskQuota)
				}
				if *cfg.Instances != 2 {
					t.Errorf("expected 2 instances, got %d", *cfg.Instances)
				}
				if *cfg.Memory != "256M" {
					t.Errorf("expected memory '256M', got %s", *cfg.Memory)
				}
				if *cfg.Timeout != 30 {
					t.Errorf("expected timeout 30, got %d", *cfg.Timeout)
				}
				if cfg.HealthCheckHTTPEndpoint != nil {
					t.Errorf("expected nil health check endpoint for empty value, got %v", *cfg.HealthCheckHTTPEndpoint)
				}
				if *cfg.HealthCheckInterval != 10 {
					t.Errorf("expected health check interval 10, got %d", *cfg.HealthCheckInterval)
				}
				if cfg.HealthCheckInvocationTimeout != nil {
					t.Errorf("expected nil health check invocation timeout for zero value, got %v", *cfg.HealthCheckInvocationTimeout)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertProcessConfiguration(&tt.process)
			if tt.check != nil {
				tt.check(t, result)
			}
		})
	}
}

func TestConvertReadinessHealthCheckConfiguration(t *testing.T) {
	tests := []struct {
		name        string
		appManifest *operation.AppManifest
		check       func(t *testing.T, cfg *v1alpha1.ReadinessHealthCheckConfiguration)
	}{
		{
			name:        "nil manifest returns empty config",
			appManifest: nil,
			check: func(t *testing.T, cfg *v1alpha1.ReadinessHealthCheckConfiguration) {
				if cfg.ReadinessHealthCheckType != nil {
					t.Errorf("expected nil type for nil manifest, got %v", *cfg.ReadinessHealthCheckType)
				}
				if cfg.ReadinessHealthCheckHTTPEndpoint != nil {
					t.Errorf("expected nil endpoint for nil manifest, got %v", *cfg.ReadinessHealthCheckHTTPEndpoint)
				}
				if cfg.ReadinessHealthCheckInterval != nil {
					t.Errorf("expected nil interval for nil manifest, got %v", *cfg.ReadinessHealthCheckInterval)
				}
				if cfg.ReadinessHealthCheckInvocationTimeout != nil {
					t.Errorf("expected nil timeout for nil manifest, got %v", *cfg.ReadinessHealthCheckInvocationTimeout)
				}
			},
		},
		{
			name: "empty manifest returns empty config",
			appManifest: &operation.AppManifest{
				AppManifestProcess: operation.AppManifestProcess{
					ReadinessHealthCheckType:         "",
					ReadinessHealthCheckHttpEndpoint: "",
					ReadinessHealthCheckInterval:     0,
					ReadinessHealthInvocationTimeout: 0,
				},
			},
			check: func(t *testing.T, cfg *v1alpha1.ReadinessHealthCheckConfiguration) {
				if cfg.ReadinessHealthCheckType != nil {
					t.Errorf("expected nil type for empty value, got %v", *cfg.ReadinessHealthCheckType)
				}
				if cfg.ReadinessHealthCheckHTTPEndpoint != nil {
					t.Errorf("expected nil endpoint for empty value, got %v", *cfg.ReadinessHealthCheckHTTPEndpoint)
				}
				if cfg.ReadinessHealthCheckInterval != nil {
					t.Errorf("expected nil interval for zero value, got %v", *cfg.ReadinessHealthCheckInterval)
				}
				if cfg.ReadinessHealthCheckInvocationTimeout != nil {
					t.Errorf("expected nil timeout for zero value, got %v", *cfg.ReadinessHealthCheckInvocationTimeout)
				}
			},
		},
		{
			name: "all fields set",
			appManifest: &operation.AppManifest{
				AppManifestProcess: operation.AppManifestProcess{
					ReadinessHealthCheckType:         "http",
					ReadinessHealthCheckHttpEndpoint: "/ready",
					ReadinessHealthCheckInterval:     30,
					ReadinessHealthInvocationTimeout: 5,
				},
			},
			check: func(t *testing.T, cfg *v1alpha1.ReadinessHealthCheckConfiguration) {
				if cfg.ReadinessHealthCheckType == nil || *cfg.ReadinessHealthCheckType != "http" {
					t.Errorf("expected type 'http', got %v", cfg.ReadinessHealthCheckType)
				}
				if cfg.ReadinessHealthCheckHTTPEndpoint == nil || *cfg.ReadinessHealthCheckHTTPEndpoint != "/ready" {
					t.Errorf("expected endpoint '/ready', got %v", cfg.ReadinessHealthCheckHTTPEndpoint)
				}
				if cfg.ReadinessHealthCheckInterval == nil || *cfg.ReadinessHealthCheckInterval != 30 {
					t.Errorf("expected interval 30, got %v", cfg.ReadinessHealthCheckInterval)
				}
				if cfg.ReadinessHealthCheckInvocationTimeout == nil || *cfg.ReadinessHealthCheckInvocationTimeout != 5 {
					t.Errorf("expected timeout 5, got %v", cfg.ReadinessHealthCheckInvocationTimeout)
				}
			},
		},
		{
			name: "partial fields set - only type",
			appManifest: &operation.AppManifest{
				AppManifestProcess: operation.AppManifestProcess{
					ReadinessHealthCheckType: "port",
				},
			},
			check: func(t *testing.T, cfg *v1alpha1.ReadinessHealthCheckConfiguration) {
				if cfg.ReadinessHealthCheckType == nil || *cfg.ReadinessHealthCheckType != "port" {
					t.Errorf("expected type 'port', got %v", cfg.ReadinessHealthCheckType)
				}
				if cfg.ReadinessHealthCheckHTTPEndpoint != nil {
					t.Errorf("expected nil endpoint for empty value, got %v", *cfg.ReadinessHealthCheckHTTPEndpoint)
				}
				if cfg.ReadinessHealthCheckInterval != nil {
					t.Errorf("expected nil interval for zero value, got %v", *cfg.ReadinessHealthCheckInterval)
				}
				if cfg.ReadinessHealthCheckInvocationTimeout != nil {
					t.Errorf("expected nil timeout for zero value, got %v", *cfg.ReadinessHealthCheckInvocationTimeout)
				}
			},
		},
		{
			name: "partial fields set - only interval and timeout",
			appManifest: &operation.AppManifest{
				AppManifestProcess: operation.AppManifestProcess{
					ReadinessHealthCheckInterval:     10,
					ReadinessHealthInvocationTimeout: 2,
				},
			},
			check: func(t *testing.T, cfg *v1alpha1.ReadinessHealthCheckConfiguration) {
				if cfg.ReadinessHealthCheckType != nil {
					t.Errorf("expected nil type for empty value, got %v", *cfg.ReadinessHealthCheckType)
				}
				if cfg.ReadinessHealthCheckHTTPEndpoint != nil {
					t.Errorf("expected nil endpoint for empty value, got %v", *cfg.ReadinessHealthCheckHTTPEndpoint)
				}
				if cfg.ReadinessHealthCheckInterval == nil || *cfg.ReadinessHealthCheckInterval != 10 {
					t.Errorf("expected interval 10, got %v", cfg.ReadinessHealthCheckInterval)
				}
				if cfg.ReadinessHealthCheckInvocationTimeout == nil || *cfg.ReadinessHealthCheckInvocationTimeout != 2 {
					t.Errorf("expected timeout 2, got %v", cfg.ReadinessHealthCheckInvocationTimeout)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertReadinessHealthCheckConfiguration(tt.appManifest)
			if tt.check != nil {
				tt.check(t, result)
			}
		})
	}
}

// Note: convertAppResource is tested through integration-style tests
// since it calls getAppManifest which makes external API calls.
// The helper functions (convertDockerField, convertProcessesField, convertProcessConfiguration,
// convertReadinessHealthCheckConfiguration) are unit tested above.
