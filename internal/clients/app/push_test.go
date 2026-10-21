package app

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
)

func TestNewManifestFromSpec(t *testing.T) {
	tests := []struct {
		name      string
		spec      v1alpha1.AppParameters
		wantImage string
		wantErr   bool
	}{
		{
			name: "docker lifecycle uses the configured image",
			spec: v1alpha1.AppParameters{
				Name:      "test-app",
				Lifecycle: "docker",
				Docker:    &v1alpha1.DockerConfiguration{Image: "nginx:1.21"},
			},
			wantImage: "nginx:1.21",
		},
		{
			name: "omitted lifecycle is treated as docker",
			spec: v1alpha1.AppParameters{
				Name:   "test-app",
				Docker: &v1alpha1.DockerConfiguration{Image: "nginx:1.21"},
			},
			wantImage: "nginx:1.21",
		},
		{
			name: "missing docker config returns an error",
			spec: v1alpha1.AppParameters{
				Name:      "test-app",
				Lifecycle: "docker",
			},
			wantErr: true,
		},
		{
			name: "empty docker image returns an error",
			spec: v1alpha1.AppParameters{
				Name:   "test-app",
				Docker: &v1alpha1.DockerConfiguration{},
			},
			wantErr: true,
		},
		{
			name: "buildpack lifecycle is rejected",
			spec: v1alpha1.AppParameters{
				Name:      "test-app",
				Lifecycle: "buildpack",
				Docker:    &v1alpha1.DockerConfiguration{Image: "nginx:1.21"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest, err := newManifestFromSpec(tt.spec, nil)
			if (err != nil) != tt.wantErr {
				t.Fatalf("newManifestFromSpec() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if manifest != nil {
					t.Errorf("expected no manifest on error, got %v", manifest)
				}
				return
			}
			if manifest.Docker == nil {
				t.Fatal("expected docker section in manifest")
			}
			if diff := cmp.Diff(tt.wantImage, manifest.Docker.Image); diff != "" {
				t.Errorf("docker image mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
