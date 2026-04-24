//go:build upgrade

//
// This file (space_quota_external_name_upgrade_test.go) contains Test_Space_Quota_External_Name,
// which validates that SpaceQuota resources maintain proper external-name formatting
// during provider upgrades. Specifically, it verifies:
//   - External-name annotation exists and follows UUID format
//   - External-name value remains unchanged after provider upgrade
//
// This test demonstrates the use of CustomUpgradeTestBuilder for creating
// specialized upgrade tests with custom pre/post-upgrade validation logic.

package upgrade

import (
	"context"
	"testing"

	v1alpha1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/test"
	"k8s.io/klog/v2"
	res "sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

var (
	spaceQuotaCustomResourceDirectories = []string{
		"./testdata/customCrs/externalNames/import",
		"./testdata/customCrs/externalNames/spaceQuota",
	}
)

func Test_Space_Quota_External_Name(t *testing.T) {
	const spaceQuotaName = "upgrade-test-external-name-space-quota"

	upgradeTest := NewCustomUpgradeTest("space-quota-external-name-test").
		FromVersion(fromTag).
		ToVersion(toTag).
		WithResourceDirectories(spaceQuotaCustomResourceDirectories).
		WithCustomPreUpgradeAssessment(
			"Verify external name before upgrade",
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				r, err := res.New(cfg.Client().RESTConfig())
				if err != nil {
					t.Fatalf("Failed to create resource client: %v", err)
				}

				err = v1alpha1.SchemeBuilder.AddToScheme(r.GetScheme())
				if err != nil {
					t.Fatalf("Failed to add CloudFoundry scheme: %v", err)
				}

				spaceQuota := &v1alpha1.SpaceQuota{}

				err = r.Get(ctx, spaceQuotaName, cfg.Namespace(), spaceQuota)
				if err != nil {
					t.Fatalf("Failed to get SpaceQuota resource: %v", err)
				}

				annotations := spaceQuota.GetAnnotations()
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
				spaceQuota := &v1alpha1.SpaceQuota{}
				r := cfg.Client().Resources()

				err := r.Get(ctx, spaceQuotaName, cfg.Namespace(), spaceQuota)
				if err != nil {
					t.Fatalf("Failed to get SpaceQuota resource: %v", err)
				}

				annotations := spaceQuota.GetAnnotations()
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
