//go:build upgrade

package upgrade

import (
	"context"
	"testing"

	v1alpha1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/test"
	"k8s.io/klog/v2"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

var (
	customResourceDirectories = []string{
		"./testdata/customCRs/spaceExternalName",
	}
)

func Test_Space_External_Name(t *testing.T) {
	const spaceName = "upgrade-test-space"

	fromTag, toTag := loadTags()

	upgradeTest := NewCustomUpgradeTest("space-external-name-test").
		FromVersion(fromTag).
		ToVersion(toTag).
		WithResourceDirectories(customResourceDirectories).
		WithCustomPreUpgradeAssessment(
			"Verify external name before upgrade",
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				space := &v1alpha1.Space{}
				r := cfg.Client().Resources()

				err := r.Get(ctx, spaceName, cfg.Namespace(), space)
				if err != nil {
					t.Fatalf("Failed to get Space resource: %v", err)
				}

				annotations := space.GetAnnotations()
				externalName, exists := annotations["crossplane.io/external-name"]
				if !exists {
					t.Fatal("External name annotation does not exist")
				}

				klog.V(4).Infof("Pre-upgrade external name: %s", externalName)

				if !test.UUIDRegex.MatchString(externalName) {
					t.Fatalf("External name '%s' does not match expected UUID format", externalName)
				}

				return context.WithValue(ctx, "preUpgradeExternalName", externalName)
			},
		).
		WithCustomPostUpgradeAssessment(
			"Verify external name after upgrade",
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				space := &v1alpha1.Space{}
				r := cfg.Client().Resources()

				err := r.Get(ctx, spaceName, cfg.Namespace(), space)
				if err != nil {
					t.Fatalf("Failed to get Space resource: %v", err)
				}

				annotations := space.GetAnnotations()
				externalName, exists := annotations["crossplane.io/external-name"]
				if !exists {
					t.Fatal("External name annotation does not exist after upgrade")
				}

				klog.V(4).Infof("Post-upgrade external name: %s", externalName)

				if !test.UUIDRegex.MatchString(externalName) {
					t.Fatalf("External name '%s' does not match expected UUID format after upgrade", externalName)
				}

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

	testenv.Test(t, upgradeTest.Feature())
}

func loadTags() (string, string) {
	// Reuse from upgrade_test.go or define here
	return fromTag, toTag
}
