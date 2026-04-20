//go:build upgrade

package upgrade

import (
	"context"
	"strings"
	"testing"

	v1alpha1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients"
	"k8s.io/klog/v2"
	res "sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

func Test_OrgMembers_External_Name(t *testing.T) {
	const (
		orgMembersName   = "upgrade-test-org-members"
		expectedRoleType = "Manager" // canonicalized singular form
	)

	upgradeTest := NewCustomUpgradeTest("org-members-external-name-test").
		FromVersion(fromTag).
		ToVersion(toTag).
		WithResourceDirectories(customResourceDirectories).
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

				orgMembers := &v1alpha1.OrgMembers{}
				err = r.Get(ctx, orgMembersName, cfg.Namespace(), orgMembers)
				if err != nil {
					t.Fatalf("Failed to get OrgMembers resource: %v", err)
				}

				externalName, exists := orgMembers.GetAnnotations()["crossplane.io/external-name"]
				if !exists {
					t.Fatal("External name annotation does not exist before upgrade")
				}

				klog.V(4).Infof("Pre-upgrade external name: %s", externalName)

				// Legacy format: RoleType@OrgGUID (e.g. Managers@abc123-def456-...)
				if !isLegacyOrgMemberExternalName(externalName) {
					t.Fatalf("Pre-upgrade external name '%s' does not match expected legacy format RoleType@OrgGUID", externalName)
				}

				return context.WithValue(ctx, "preUpgradeOrgMemberExternalName", externalName)
			},
		).
		WithCustomPostUpgradeAssessment(
			"Verify compound external name after upgrade",
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				orgMembers := &v1alpha1.OrgMembers{}
				r := cfg.Client().Resources()

				err := r.Get(ctx, orgMembersName, cfg.Namespace(), orgMembers)
				if err != nil {
					t.Fatalf("Failed to get OrgMembers resource: %v", err)
				}

				externalName := assertOrgMemberCompoundExternalName(t, orgMembers.GetAnnotations(), expectedRoleType, "after upgrade")
				klog.V(4).Infof("Post-upgrade external name: %s", externalName)

				// Verify the migrated compound key preserves the same org GUID and role type from the pre-upgrade legacy external-name
				preUpgradeExternalName, _ := ctx.Value("preUpgradeOrgMemberExternalName").(string)
				if preUpgradeExternalName == "" {
					t.Fatal("Pre-upgrade external name not found in context")
				}
				legacyParts := strings.SplitN(preUpgradeExternalName, "@", 2)
				if len(legacyParts) != 2 {
					t.Fatalf("Pre-upgrade external name '%s' is not in expected legacy format RoleType@OrgGUID", preUpgradeExternalName)
				}
				preUpgradeOrgGUID := legacyParts[1]

				// Derive expected compound key: <same-org-guid>/<canonical-role-type>
				expectedExternalName := preUpgradeOrgGUID + "/" + expectedRoleType
				if externalName != expectedExternalName {
					t.Fatalf("Post-upgrade external name '%s' does not match expected '%s' derived from pre-upgrade legacy name '%s'", externalName, expectedExternalName, preUpgradeExternalName)
				}

				// Also verify the role type was canonicalized (e.g. Managers -> Manager)
				compoundParts := strings.SplitN(externalName, "/", 2)
				if compoundParts[1] != expectedRoleType {
					t.Fatalf("Role type '%s' in external name '%s' does not match expected canonical role type '%s'", compoundParts[1], externalName, expectedRoleType)
				}

				klog.V(4).Info("External name migration validation passed: legacy format migrated to compound format with preserved identity")
				return ctx
			},
		)

	testenv.Test(t, upgradeTest.Feature())
}

// isLegacyOrgMemberExternalName checks if the external name follows the legacy format RoleType@OrgGUID
func isLegacyOrgMemberExternalName(externalName string) bool {
	parts := strings.SplitN(externalName, "@", 2)
	if len(parts) != 2 {
		return false
	}
	// The part after @ should be a valid UUID (org GUID)
	return clients.IsValidGUID(parts[1])
}

// assertOrgMemberCompoundExternalName validates that the external name follows the compound format <org-guid>/<role-type>
func assertOrgMemberCompoundExternalName(t *testing.T, annotations map[string]string, expectedRoleType string, phase string) string {
	t.Helper()

	externalName, exists := annotations["crossplane.io/external-name"]
	if !exists {
		t.Fatalf("External name annotation does not exist %s", phase)
	}

	parts := strings.SplitN(externalName, "/", 2)
	if len(parts) != 2 {
		t.Fatalf("External name '%s' does not contain expected compound key separator %s", externalName, phase)
	}

	if !clients.IsValidGUID(parts[0]) {
		t.Fatalf("Org GUID '%s' in external name '%s' is not a valid UUID %s", parts[0], externalName, phase)
	}

	if parts[1] != expectedRoleType {
		t.Fatalf("Role type '%s' in external name '%s' does not match expected role type '%s' %s", parts[1], externalName, expectedRoleType, phase)
	}

	return externalName
}
