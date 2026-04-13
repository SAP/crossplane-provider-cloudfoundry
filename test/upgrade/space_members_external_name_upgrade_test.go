//go:build upgrade

package upgrade

import (
	"context"
	"regexp"
	"strings"
	"testing"

	v1alpha1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/test"
	"k8s.io/klog/v2"
	res "sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

var CompoundExternalNameRegex = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}/(Developer|Auditor|Manager|Supporter|Developers|Auditors|Managers|Supporters)$`)

func Test_SpaceMembers_External_Name(t *testing.T) {
	const (
		spaceMembersName = "upgrade-test-space-members"
		expectedRoleType = "Developers"
	)

	upgradeTest := NewCustomUpgradeTest("space-members-external-name-test").
		FromVersion(fromTag).
		ToVersion(toTag).
		WithResourceDirectories(customResourceDirectories).
		WithCustomPreUpgradeAssessment(
			"Verify compound external name before upgrade",
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

				externalName := assertCompoundExternalName(t, spaceMembers.GetAnnotations(), expectedRoleType, "before upgrade")
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

				if externalName != preUpgradeExternalName {
					t.Fatalf("External name changed during upgrade: before='%s', after='%s'",
						preUpgradeExternalName, externalName)
				}

				klog.V(4).Info("External name validation passed: compound format correct and unchanged")
				return ctx
			},
		)

	testenv.Test(t, upgradeTest.Feature())
}

func assertCompoundExternalName(t *testing.T, annotations map[string]string, expectedRoleType string, phase string) string {
	t.Helper()

	externalName, exists := annotations["crossplane.io/external-name"]
	if !exists {
		t.Fatalf("External name annotation does not exist %s", phase)
	}

	if !CompoundExternalNameRegex.MatchString(externalName) {
		t.Fatalf("External name '%s' does not match expected compound format %s", externalName, phase)
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
