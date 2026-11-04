package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	resourcev1alpha1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/crossplane/crossplane/apis/v2/core/v2"
	"sigs.k8s.io/yaml"
)

const rotationFixtureMaxXSAppNameLength = 100

func TestRotationFixturesUseIsolatedXSUAAInstances(t *testing.T) {
	for _, buildID := range []string{"build-123", "build-1234567890"} {
		t.Run(buildID, func(t *testing.T) {
			noTTL := loadRotationFixtureSet(t, "no-ttl", buildID)
			explicitTTL := loadRotationFixtureSet(t, "explicit-ttl", buildID)

			assertRotationImportFixture(t, noTTL.org, noTTL.space, "rotation-org-"+buildID, "rotation-space-"+buildID)
			assertRotationImportFixture(t, explicitTTL.org, explicitTTL.space, "cleanup-org-"+buildID, "cleanup-space-"+buildID)
			assertRotationInstanceFixture(t, noTTL.instance, "rotation-instance-"+buildID, "scb-rotation-no-ttl-"+buildID, "rotation-space-"+buildID)
			assertRotationInstanceFixture(t, explicitTTL.instance, "cleanup-instance-"+buildID, "scb-rotation-cleanup-"+buildID, "cleanup-space-"+buildID)
			assertRotationBindingFixture(t, noTTL.binding, "scb-rotation-no-ttl-"+buildID, "rotation-instance-"+buildID, "scb-rotation-no-ttl-secret-"+buildID, false)
			assertRotationBindingFixture(t, explicitTTL.binding, "scb-rotation-cleanup-"+buildID, "cleanup-instance-"+buildID, "scb-rotation-cleanup-secret-"+buildID, true)

			if noTTL.instance.Spec.ForProvider.Name == nil || explicitTTL.instance.Spec.ForProvider.Name == nil || *noTTL.instance.Spec.ForProvider.Name == *explicitTTL.instance.Spec.ForProvider.Name {
				t.Fatal("rotation tests must own distinct Cloud Foundry service-instance names")
			}
			noTTLXSAppName := rotationXSUAAAppName(t, noTTL.instance)
			explicitXSAppName := rotationXSUAAAppName(t, explicitTTL.instance)
			if noTTLXSAppName == explicitXSAppName {
				t.Fatal("rotation tests must use distinct XSUAA application identities")
			}
			for _, xsappname := range []string{noTTLXSAppName, explicitXSAppName} {
				if len(xsappname) > rotationFixtureMaxXSAppNameLength {
					t.Fatalf("XSUAA xsappname length %d exceeds the documented limit", len(xsappname))
				}
			}
		})
	}
}

type rotationFixtureSet struct {
	org      *resourcev1alpha1.Organization
	space    *resourcev1alpha1.Space
	instance *resourcev1alpha1.ServiceInstance
	binding  *resourcev1alpha1.ServiceCredentialBinding
}

func loadRotationFixtureSet(t *testing.T, scenario, buildID string) rotationFixtureSet {
	t.Helper()
	base := filepath.Join("e2e", "crs", "servicecredentialbinding-rotation", scenario)
	instanceYAML := renderRotationFixture(t, filepath.Join(base, "serviceinstance.yaml"), buildID)
	var instance resourcev1alpha1.ServiceInstance
	if err := yaml.Unmarshal([]byte(instanceYAML), &instance); err != nil {
		t.Fatalf("decode %s service-instance fixture: %v", scenario, err)
	}
	bindingYAML := renderRotationFixture(t, filepath.Join(base, "scb.yaml"), buildID)
	var binding resourcev1alpha1.ServiceCredentialBinding
	if err := yaml.Unmarshal([]byte(bindingYAML), &binding); err != nil {
		t.Fatalf("decode %s binding fixture: %v", scenario, err)
	}
	imports := renderRotationFixture(t, filepath.Join(base, "import.yaml"), buildID)
	org, space := decodeRotationImportFixtures(t, imports)
	return rotationFixtureSet{org: org, space: space, instance: &instance, binding: &binding}
}

func renderRotationFixture(t *testing.T, path, buildID string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read rotation fixture %q: %v", path, err)
	}
	return strings.ReplaceAll(string(contents), "$BUILD_ID", buildID)
}

func decodeRotationImportFixtures(t *testing.T, documents string) (*resourcev1alpha1.Organization, *resourcev1alpha1.Space) {
	t.Helper()
	var org *resourcev1alpha1.Organization
	var space *resourcev1alpha1.Space
	for _, document := range strings.Split(documents, "---") {
		if strings.TrimSpace(document) == "" {
			continue
		}
		var header struct {
			Kind string `json:"kind"`
		}
		jsonDocument, err := yaml.YAMLToJSON([]byte(document))
		if err != nil {
			t.Fatalf("decode rotation import YAML document: %v", err)
		}
		if err := json.Unmarshal(jsonDocument, &header); err != nil {
			t.Fatalf("decode rotation import document kind: %v", err)
		}
		switch header.Kind {
		case "Organization":
			org = &resourcev1alpha1.Organization{}
			if err := json.Unmarshal(jsonDocument, org); err != nil {
				t.Fatalf("decode Organization import fixture: %v", err)
			}
		case "Space":
			space = &resourcev1alpha1.Space{}
			if err := json.Unmarshal(jsonDocument, space); err != nil {
				t.Fatalf("decode Space import fixture: %v", err)
			}
		default:
			t.Fatalf("unexpected kind %q in rotation import fixture", header.Kind)
		}
	}
	if org == nil || space == nil {
		t.Fatal("rotation import fixture must contain one Organization and one Space")
	}
	return org, space
}

func assertRotationImportFixture(t *testing.T, org *resourcev1alpha1.Organization, space *resourcev1alpha1.Space, orgName, spaceName string) {
	t.Helper()
	if org.Name != orgName || org.Namespace != "default" || org.Spec.ForProvider.Name != "cf-ci-e2e" || !onlyObserve(org.Spec.ManagementPolicies) {
		t.Fatal("rotation Organization import fixture must preserve the expected owned wrapper, namespace, CF identity, and Observe-only policy")
	}
	if space.Name != spaceName || space.Namespace != "default" || space.Spec.ForProvider.Name != "space-donotdelete" || !onlyObserve(space.Spec.ManagementPolicies) {
		t.Fatal("rotation Space import fixture must preserve the expected owned wrapper, namespace, CF identity, and Observe-only policy")
	}
	if space.Spec.ForProvider.OrgRef == nil || space.Spec.ForProvider.OrgRef.Name != orgName || space.Spec.ForProvider.OrgRef.Policy == nil || space.Spec.ForProvider.OrgRef.Policy.Resolve == nil || *space.Spec.ForProvider.OrgRef.Policy.Resolve != v2.ResolvePolicyAlways {
		t.Fatal("rotation Space import fixture must resolve through its matching Organization wrapper")
	}
}

func assertRotationInstanceFixture(t *testing.T, instance *resourcev1alpha1.ServiceInstance, name, xsappname, spaceName string) {
	t.Helper()
	if instance.Name != name || instance.Namespace != "default" || instance.Spec.ForProvider.Type != resourcev1alpha1.ManagedService || instance.Spec.ForProvider.Name == nil || *instance.Spec.ForProvider.Name != name {
		t.Fatal("rotation service-instance fixture must use its scenario-specific managed resource and CF name")
	}
	plan := instance.Spec.ForProvider.Managed.ServicePlan
	if plan == nil || plan.Offering == nil || *plan.Offering != "xsuaa" || plan.Plan == nil || *plan.Plan != "application" {
		t.Fatal("rotation service-instance fixture must use the XSUAA application plan")
	}
	if instance.Spec.ForProvider.SpaceRef == nil || instance.Spec.ForProvider.SpaceRef.Name != spaceName || instance.Spec.ForProvider.SpaceRef.Policy == nil || instance.Spec.ForProvider.SpaceRef.Policy.Resolve == nil || *instance.Spec.ForProvider.SpaceRef.Policy.Resolve != v2.ResolvePolicyAlways {
		t.Fatal("rotation service-instance fixture must resolve its matching Space wrapper")
	}
	parameters := rotationXSUAAParameters(t, instance)
	if parameters["xsappname"] != xsappname || parameters["tenant-mode"] != "dedicated" {
		t.Fatal("rotation XSUAA fixture must have its unique application name and dedicated tenant mode")
	}
	oauth, ok := parameters["oauth2-configuration"].(map[string]any)
	if !ok {
		t.Fatal("rotation XSUAA fixture must specify oauth2-configuration")
	}
	credentialTypes, ok := oauth["credential-types"].([]any)
	if !ok || len(credentialTypes) != 1 || credentialTypes[0] != "binding-secret" {
		t.Fatal("rotation XSUAA fixture must explicitly select binding-secret credentials")
	}
}

func rotationXSUAAParameters(t *testing.T, instance *resourcev1alpha1.ServiceInstance) map[string]any {
	t.Helper()
	if instance.Spec.ForProvider.Managed.Parameters == nil || len(instance.Spec.ForProvider.Managed.Parameters.Raw) == 0 {
		t.Fatal("rotation XSUAA fixture must define non-secret broker parameters")
	}
	var parameters map[string]any
	if err := json.Unmarshal(instance.Spec.ForProvider.Managed.Parameters.Raw, &parameters); err != nil {
		t.Fatalf("decode rotation XSUAA broker parameters: %v", err)
	}
	return parameters
}

func rotationXSUAAAppName(t *testing.T, instance *resourcev1alpha1.ServiceInstance) string {
	t.Helper()
	name, ok := rotationXSUAAParameters(t, instance)["xsappname"].(string)
	if !ok || name == "" {
		t.Fatal("rotation XSUAA fixture must set a nonempty xsappname")
	}
	return name
}

func assertRotationBindingFixture(t *testing.T, binding *resourcev1alpha1.ServiceCredentialBinding, name, instanceName, secretName string, explicitTTL bool) {
	t.Helper()
	if binding.Name != name || binding.Namespace != "default" || binding.Spec.ForProvider.Name == nil || *binding.Spec.ForProvider.Name != name || binding.Spec.ForProvider.Type != "key" {
		t.Fatal("rotation binding fixture must preserve its scenario-specific key identity")
	}
	if _, exists := binding.Annotations["servicecredentialbinding.cloudfoundry.crossplane.io/force-rotation"]; exists {
		t.Fatal("rotation binding fixture must not commit an operational force-rotation annotation")
	}
	ref := binding.Spec.ForProvider.ServiceInstanceRef
	if ref == nil || ref.Name != instanceName || ref.Policy == nil || ref.Policy.Resolve == nil || *ref.Policy.Resolve != v2.ResolvePolicyAlways {
		t.Fatal("rotation binding fixture must resolve its matching owned service instance")
	}
	secret := binding.Spec.WriteConnectionSecretToReference
	if secret == nil || secret.Name != secretName || secret.Namespace != "default" {
		t.Fatal("rotation binding fixture must publish to its scenario-specific Secret")
	}
	if explicitTTL {
		rotation := binding.Spec.ForProvider.Rotation
		if rotation == nil || rotation.Frequency == nil || rotation.Frequency.Duration.String() != "5m0s" || rotation.TTL == nil || rotation.TTL.Duration.String() != "5m0s" {
			t.Fatal("explicit-TTL rotation policy must remain frequency=5m and ttl=5m")
		}
	} else if binding.Spec.ForProvider.Rotation != nil {
		t.Fatal("no-TTL rotation fixture must continue to omit scheduled rotation")
	}
}

func onlyObserve(policies v2.ManagementPolicies) bool {
	return len(policies) == 1 && policies[0] == v2.ManagementActionObserve
}
