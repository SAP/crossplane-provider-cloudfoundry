//go:build ignore

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestServiceInstanceCleanupNames(t *testing.T) {
	for _, suffix := range []string{"-411-1", "-411-2"} {
		t.Run(suffix, func(t *testing.T) {
			got := serviceInstanceCleanupNames(suffix)
			want := []string{
				"e2e-service-instance" + suffix,
				"e2e-ups" + suffix,
				"e2e-ups-no-credentials" + suffix,
				"e2e-serviceroutebinding-serviceinstance" + suffix,
				"rotation-instance" + suffix,
				"cleanup-instance" + suffix,
			}
			require.Equal(t, want, got)
			unique := make(map[string]struct{}, len(got))
			for _, name := range got {
				_, alreadySeen := unique[name]
				require.False(t, alreadySeen, "duplicate cleanup name %q", name)
				unique[name] = struct{}{}
			}
			otherSuffix := "-411-1"
			if suffix == otherSuffix {
				otherSuffix = "-411-2"
			}
			for _, name := range got {
				require.NotContains(t, name, otherSuffix)
			}
			require.NotContains(t, got, "cf-ci-e2e")
			require.NotContains(t, got, "space-donotdelete")
		})
	}
}

func TestRotationCleanupNamesMatchFixtures(t *testing.T) {
	fixtures := []string{
		"../../test/e2e/crs/servicecredentialbinding-rotation/no-ttl/serviceinstance.yaml",
		"../../test/e2e/crs/servicecredentialbinding-rotation/explicit-ttl/serviceinstance.yaml",
	}

	for _, fixture := range fixtures {
		t.Run(filepath.Base(filepath.Dir(fixture)), func(t *testing.T) {
			data, err := os.ReadFile(fixture)
			require.NoError(t, err)
			manifest := strings.ReplaceAll(string(data), "$BUILD_ID", "411-fixture")

			var resource struct {
				Spec struct {
					ForProvider struct {
						Name string `json:"name"`
					} `json:"forProvider"`
				} `json:"spec"`
			}
			require.NoError(t, yaml.Unmarshal([]byte(manifest), &resource))
			require.NotEmpty(t, resource.Spec.ForProvider.Name)
			require.Contains(t, serviceInstanceCleanupNames("-411-fixture"), resource.Spec.ForProvider.Name)
		})
	}
}
