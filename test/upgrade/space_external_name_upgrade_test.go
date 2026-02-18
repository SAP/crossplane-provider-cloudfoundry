//go:build upgrade

package upgrade

import (
	"context"
	"testing"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/cloudfoundry/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/test"
	"k8s.io/klog/v2"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

var (
	// Format should match what you use in main_test.go (fromPackage/toPackage)
	fromCustomTag = "0.3.0" // Reuse from main_test.go
	toCustomTag   = "0.3.1" // Reuse from main_test.go

	customResourceDirectories = []string{
		"./testdata/customCRs/externalNames",
	}
)

func Test_Space_External_Name(t *testing.T) {
	const spaceName = "upgrade-test-space"

	upgradeTest := test.NewCustomUpgradeTest("space-external-name-test", fromCustomTag, toCustomTag).
		WithResourceDirectories(customResourceDirectories).
		PreUpgradeAssessment(
			"verify external name before upgrade",
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				space := &v1alpha1.Space{}
				r := cfg.Client().Resources()

				err := r.Get(ctx, spaceName, cfg.Namespace(), space)
				if err != nil {
					t.Fatalf("Failed to get Space resource: %v", err)
				}

				// Get the external name annotation
				annotations := space.GetAnnotations()
				externalName, exists := annotations["crossplane.io/external-name"]
				if !exists {
					t.Fatal("External name annotation does not exist")
				}

				klog.V(4).Infof("Pre-upgrade external name: %s", externalName)

				// Verify external name matches UUID format
				if !test.UUIDRegex.MatchString(externalName) {
					t.Fatalf("External name '%s' does not match expected UUID format", externalName)
				}

				// Store the external name in context for post-upgrade verification
				return context.WithValue(ctx, "preUpgradeExternalName", externalName)
			},
		).
		PostUpgradeAssessment(
			"verify external name after upgrade",
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				space := &v1alpha1.Space{}
				r := cfg.Client().Resources()

				err := r.Get(ctx, spaceName, cfg.Namespace(), space)
				if err != nil {
					t.Fatalf("Failed to get Space resource: %v", err)
				}

				// Get the external name annotation
				annotations := space.GetAnnotations()
				externalName, exists := annotations["crossplane.io/external-name"]
				if !exists {
					t.Fatal("External name annotation does not exist after upgrade")
				}

				klog.V(4).Infof("Post-upgrade external name: %s", externalName)

				// Verify external name matches UUID format
				if !test.UUIDRegex.MatchString(externalName) {
					t.Fatalf("External name '%s' does not match expected UUID format after upgrade", externalName)
				}

				// Verify external name hasn't changed during upgrade
				preUpgradeExternalName, ok := ctx.Value("preUpgradeExternalName").(string)
				if !ok {
					t.Fatal("Failed to retrieve pre-upgrade external name from context")
				}

				if externalName != preUpgradeExternalName {
					t.Fatalf("External name changed during upgrade: before='%s', after='%s'",
						preUpgradeExternalName, externalName)
				}

				klog.V(4).Info("External name validation passed: format correct and unchanged")
				return ctx
			},
		)

	upgradeTest.Run(t)
}
