//go:build upgrade

package upgrade

import (
	"context"
	"strings"
	"testing"

	v1alpha1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/test"
	"k8s.io/klog/v2"
	res "sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

var spaceMembersCustomResourceDirectories = []string{
	"./testdata/customCrs/externalNames/import",
	"./testdata/customCrs/externalNames/space",
	"./testdata/customCrs/externalNames/spaceMembers",
}

func Test_SpaceMembers_External_Name(t *testing.T) {
	const (
		spaceMembersName = "upgrade-test-space-members"
		expectedRoleType = "Developer"
	)

	upgradeTest := NewCustomUpgradeTest("space-members-external-name-test").
		FromVersion(fromTag).
		ToVersion(toTag).
		WithResourceDirectories(spaceMembersCustomResourceDirectories).
		WithCustomPreUpgradeAssessment(
			"Verify legacy external name before upgrade",
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				r, err := res.New(cfg.Client().RESTConfig())
				if err != nil {
					t.Fatalf("Failed to create resource client: %v", err)
				}

				err = v1alpha1.SchemeBuilder.AddToScheme(r.GetScheme())
				if err != nil {
					t.Fatalf("Failed to add CloudFoundry scheme: %v", err)
				}

				spaceMembers := &v1alpha1.SpaceMembers{}
				err = r.Get(ctx, spaceMembersName, cfg.Namespace(), spaceMembers)
				if err != nil {
					t.Fatalf("Failed to get SpaceMembers resource: %v", err)
				}

				externalName := assertLegacySpaceMembersExternalName(t, spaceMembers.GetAnnotations(), "before upgrade")
				klog.V(4).Infof("Pre-upgrade external name: %s", externalName)

				return context.WithValue(ctx, "preUpgradeExternalName", externalName)
			},
		).
		WithCustomPostUpgradeAssessment(
			"Verify compound external name after upgrade",
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				spaceMembers := &v1alpha1.SpaceMembers{}
				r := cfg.Client().Resources()

				err := r.Get(ctx, spaceMembersName, cfg.Namespace(), spaceMembers)
				if err != nil {
					t.Fatalf("Failed to get SpaceMembers resource: %v", err)
				}

				externalName := assertCompoundExternalName(t, spaceMembers.GetAnnotations(), expectedRoleType, "after upgrade")
				klog.V(4).Infof("Post-upgrade external name: %s", externalName)

				preUpgradeExternalName, ok := ctx.Value("preUpgradeExternalName").(string)
				if !ok {
					t.Fatal("Failed to retrieve pre-upgrade external name from context")
				}

				expectedExternalName := preUpgradeExternalName + "/" + expectedRoleType
				if externalName != expectedExternalName {
					t.Fatalf("External name '%s' does not match migrated value '%s' derived from legacy name '%s'",
						externalName, expectedExternalName, preUpgradeExternalName)
				}

				klog.V(4).Info("External name validation passed: legacy GUID migrated to compound format")
				return ctx
			},
		)

	testenv.Test(t, upgradeTest.Feature())
}

func assertLegacySpaceMembersExternalName(t *testing.T, annotations map[string]string, phase string) string {
	t.Helper()

	externalName, exists := annotations["crossplane.io/external-name"]
	if !exists {
		t.Fatalf("External name annotation does not exist %s", phase)
	}
	if !test.UUIDRegex.MatchString(externalName) {
		t.Fatalf("Legacy external name '%s' is not a valid GUID %s", externalName, phase)
	}

	return externalName
}

func assertCompoundExternalName(t *testing.T, annotations map[string]string, expectedRoleType string, phase string) string {
	t.Helper()

	externalName, exists := annotations["crossplane.io/external-name"]
	if !exists {
		t.Fatalf("External name annotation does not exist %s", phase)
	}

	parts := strings.SplitN(externalName, "/", 2)
	if len(parts) != 2 {
		t.Fatalf("External name '%s' does not contain expected compound key separator %s", externalName, phase)
	}

	if !test.UUIDRegex.MatchString(parts[0]) {
		t.Fatalf("Space GUID '%s' in external name '%s' is not a valid UUID %s", parts[0], externalName, phase)
	}

	if parts[1] != expectedRoleType {
		t.Fatalf("Role type '%s' in external name '%s' does not match expected role type '%s' %s", parts[1], externalName, expectedRoleType, phase)
	}

	return externalName
}
