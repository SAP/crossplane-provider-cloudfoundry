//go:build upgrade

//
// This file (service_instance_external_name_upgrade_test.go) contains Test_Service_Instance_External_Name,
// which validates that ServiceInstance resources maintain proper external-name formatting
// during provider upgrades. Specifically, it verifies:
//   - External-name annotation exists and follows UUID format
//   - External-name value remains unchanged after provider upgrade
//
// A user-provided service instance is used because it needs neither a service offering/plan nor an
// async provisioning job, keeping the upgrade flow deterministic.
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
	serviceInstanceCustomResourceDirectories = []string{
		"./testdata/customCrs/externalNames/import",
		"./testdata/customCrs/externalNames/serviceInstance",
	}
)

func Test_Service_Instance_External_Name(t *testing.T) {
	const serviceInstanceName = "upgrade-test-external-name-service-instance"

	upgradeTest := NewCustomUpgradeTest("service-instance-external-name-test").
		FromVersion(fromTag).
		ToVersion(toTag).
		WithResourceDirectories(serviceInstanceCustomResourceDirectories).
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

				serviceInstance := &v1alpha1.ServiceInstance{}

				err = r.Get(ctx, serviceInstanceName, cfg.Namespace(), serviceInstance)
				if err != nil {
					t.Fatalf("Failed to get ServiceInstance resource: %v", err)
				}

				annotations := serviceInstance.GetAnnotations()
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
				serviceInstance := &v1alpha1.ServiceInstance{}
				r := cfg.Client().Resources()

				err := r.Get(ctx, serviceInstanceName, cfg.Namespace(), serviceInstance)
				if err != nil {
					t.Fatalf("Failed to get ServiceInstance resource: %v", err)
				}

				annotations := serviceInstance.GetAnnotations()
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
