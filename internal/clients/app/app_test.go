package app

import (
	"testing"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
)

func TestDetectChanges(t *testing.T) {
	tests := []struct {
		name           string
		spec           v1alpha1.AppParameters
		status         v1alpha1.AppObservation
		expectedFields []string
	}{
		{
			name: "No changes",
			spec: v1alpha1.AppParameters{
				Name:      "test-app",
				Lifecycle: "docker",
				Docker: &v1alpha1.DockerConfiguration{
					Image: "nginx:latest",
				},
			},
			status: v1alpha1.AppObservation{
				Name:        "test-app",
				AppManifest: "applications:\n- name: test-app\n  docker:\n    image: nginx:latest",
			},
			expectedFields: []string{},
		},
		{
			name: "Docker image changed",
			spec: v1alpha1.AppParameters{
				Name:      "test-app",
				Lifecycle: "docker",
				Docker: &v1alpha1.DockerConfiguration{
					Image: "nginx:1.21",
				},
			},
			status: v1alpha1.AppObservation{
				Name:        "test-app",
				AppManifest: "applications:\n- name: test-app\n  docker:\n    image: nginx:latest",
			},
			expectedFields: []string{"docker_image"},
		},
		{
			name: "Name changed",
			spec: v1alpha1.AppParameters{
				Name:      "new-app-name",
				Lifecycle: "docker",
				Docker: &v1alpha1.DockerConfiguration{
					Image: "nginx:latest",
				},
			},
			status: v1alpha1.AppObservation{
				Name:        "test-app",
				AppManifest: "applications:\n- name: test-app\n  docker:\n    image: nginx:latest",
			},
			expectedFields: []string{"name"},
		},
		{
			name: "Both Docker image and name changed",
			spec: v1alpha1.AppParameters{
				Name:      "new-app-name",
				Lifecycle: "docker",
				Docker: &v1alpha1.DockerConfiguration{
					Image: "nginx:1.21",
				},
			},
			status: v1alpha1.AppObservation{
				Name:        "test-app",
				AppManifest: "applications:\n- name: test-app\n  docker:\n    image: nginx:latest",
			},
			expectedFields: []string{"docker_image", "name"},
		},
		{
			name: "Non-docker app name change",
			spec: v1alpha1.AppParameters{
				Name:      "new-app-name",
				Lifecycle: "buildpack",
			},
			status: v1alpha1.AppObservation{
				Name: "test-app",
			},
			expectedFields: []string{"name"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := DetectChanges(tt.spec, tt.status)
			if err != nil {
				t.Fatalf("DetectChanges() error = %v", err)
			}
			if len(result.ChangedFields) != len(tt.expectedFields) {
				t.Errorf("DetectChanges().ChangedFields length = %v, want %v", len(result.ChangedFields), len(tt.expectedFields))
			}
			for i, field := range tt.expectedFields {
				if i >= len(result.ChangedFields) || result.ChangedFields[i] != field {
					t.Errorf("DetectChanges().ChangedFields[%d] = %v, want %v", i, result.ChangedFields[i], field)
				}
			}

			// Test helper methods
			if len(tt.expectedFields) == 0 {
				if result.HasChanges() {
					t.Errorf("DetectChanges().HasChanges() = true, want false")
				}
			} else {
				if !result.HasChanges() {
					t.Errorf("DetectChanges().HasChanges() = false, want true")
				}
				// Test HasField for each expected field
				for _, field := range tt.expectedFields {
					if !result.HasField(field) {
						t.Errorf("DetectChanges().HasField(%s) = false, want true", field)
					}
				}
			}
		})
	}
}

func TestIsUpToDate(t *testing.T) {
	tests := []struct {
		name     string
		spec     v1alpha1.AppParameters
		status   v1alpha1.AppObservation
		expected bool
	}{
		{
			name: "Up to date",
			spec: v1alpha1.AppParameters{
				Name:      "test-app",
				Lifecycle: "docker",
				Docker: &v1alpha1.DockerConfiguration{
					Image: "nginx:latest",
				},
			},
			status: v1alpha1.AppObservation{
				Name:        "test-app",
				AppManifest: "applications:\n- name: test-app\n  docker:\n    image: nginx:latest",
			},
			expected: true,
		},
		{
			name: "Not up to date - Docker image changed",
			spec: v1alpha1.AppParameters{
				Name:      "test-app",
				Lifecycle: "docker",
				Docker: &v1alpha1.DockerConfiguration{
					Image: "nginx:1.21",
				},
			},
			status: v1alpha1.AppObservation{
				Name:        "test-app",
				AppManifest: "applications:\n- name: test-app\n  docker:\n    image: nginx:latest",
			},
			expected: false,
		},
		{
			name: "Not up to date - Name changed",
			spec: v1alpha1.AppParameters{
				Name:      "new-app-name",
				Lifecycle: "docker",
				Docker: &v1alpha1.DockerConfiguration{
					Image: "nginx:latest",
				},
			},
			status: v1alpha1.AppObservation{
				Name:        "test-app",
				AppManifest: "applications:\n- name: test-app\n  docker:\n    image: nginx:latest",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := IsUpToDate(tt.spec, tt.status)
			if err != nil {
				t.Fatalf("IsUpToDate() error = %v", err)
			}
			if result != tt.expected {
				t.Errorf("IsUpToDate() = %v, want %v", result, tt.expected)
			}
		})
	}
}
