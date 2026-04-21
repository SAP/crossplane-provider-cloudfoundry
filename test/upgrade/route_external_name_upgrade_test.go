//go:build upgrade

//
// This file (route_external_name_upgrade_test.go) contains Test_Route_External_Name,
// which validates that Route resources maintain proper external-name formatting
// during provider upgrades. Specifically, it verifies:
//   - External-name annotation exists and follows UUID format
//   - External-name value remains unchanged after provider upgrade

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
	routeCustomResourceDirectories = []string{
		"./testdata/customCrs/externalNames/import",
		"./testdata/customCrs/externalNames/route",
	}
)

func Test_Route_External_Name(t *testing.T) {
	const routeName = "upgrade-test-route"

	upgradeTest := NewCustomUpgradeTest("route-external-name-test").
		FromVersion(fromTag).
		ToVersion(toTag).
		WithResourceDirectories(routeCustomResourceDirectories).
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

				route := &v1alpha1.Route{}

				err = r.Get(ctx, routeName, cfg.Namespace(), route)
				if err != nil {
					t.Fatalf("Failed to get Route resource: %v", err)
				}

				annotations := route.GetAnnotations()
				externalName, exists := annotations["crossplane.io/external-name"]
				if !exists {
					t.Fatal("External name annotation does not exist")
				}

				klog.V(4).Infof("Pre-upgrade external name: %s", externalName)

				if !test.UUIDRegex.MatchString(externalName) {
					t.Fatalf("External name '%s' does not match expected UUID format", externalName)
				}

				return context.WithValue(ctx, "preUpgradeRouteExternalName", externalName)
			},
		).
		WithCustomPostUpgradeAssessment(
			"Verify external name after upgrade",
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				route := &v1alpha1.Route{}
				r := cfg.Client().Resources()

				err := r.Get(ctx, routeName, cfg.Namespace(), route)
				if err != nil {
					t.Fatalf("Failed to get Route resource: %v", err)
				}

				annotations := route.GetAnnotations()
				externalName, exists := annotations["crossplane.io/external-name"]
				if !exists {
					t.Fatal("External name annotation does not exist after upgrade")
				}

				klog.V(4).Infof("Post-upgrade external name: %s", externalName)

				if !test.UUIDRegex.MatchString(externalName) {
					t.Fatalf("External name '%s' does not match expected UUID format after upgrade", externalName)
				}

				preUpgradeExternalName, ok := ctx.Value("preUpgradeRouteExternalName").(string)
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
