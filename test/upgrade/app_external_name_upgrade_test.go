//go:build upgrade

//
// This file (app_external_name_upgrade_test.go) contains Test_App_External_Name,
// which validates that App resources maintain proper external-name formatting
// during provider upgrades. Specifically, it verifies:
//   - External-name annotation exists and follows UUID format
//   - External-name value remains unchanged after provider upgrade

package upgrade

import (
	"context"
	"testing"

	v1alpha1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/test"
	xpmeta "github.com/crossplane/crossplane-runtime/pkg/meta"
	"k8s.io/klog/v2"
	res "sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

type appPreUpgradeExternalNameKey struct{}

var (
	appExternalNameResourceDirectories = []string{
		"./testdata/customCrs/externalNames/import",
		"./testdata/customCrs/externalNames/app",
	}
)

func Test_App_External_Name(t *testing.T) {
	// defined in ./testdata/customCrs/externalNames/app/app.yaml
	const appName = "upgrade-test-app"

	upgradeTest := NewCustomUpgradeTest("app-external-name-test").
		FromVersion(fromTag).
		ToVersion(toTag).
		WithResourceDirectories(appExternalNameResourceDirectories).
		WithCustomPreUpgradeAssessment(
			"Verify external name before upgrade",
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				r, err := res.New(cfg.Client().RESTConfig())
				if err != nil {
					t.Fatalf("Failed to create resource client: %v", err)
				}
				if err = v1alpha1.SchemeBuilder.AddToScheme(r.GetScheme()); err != nil {
					t.Fatalf("Failed to add CloudFoundry scheme: %v", err)
				}

				app := &v1alpha1.App{}
				if err = r.Get(ctx, appName, cfg.Namespace(), app); err != nil {
					t.Fatalf("Failed to get App resource: %v", err)
				}

				externalName := xpmeta.GetExternalName(app)
				if !test.UUIDRegex.MatchString(externalName) {
					t.Fatalf("External name %q does not match expected UUID format", externalName)
				}

				klog.V(4).Infof("Pre-upgrade external name: %s", externalName)
				return context.WithValue(ctx, appPreUpgradeExternalNameKey{}, externalName)
			},
		).
		WithCustomPostUpgradeAssessment(
			"Verify external name after upgrade",
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				r, err := res.New(cfg.Client().RESTConfig())
				if err != nil {
					t.Fatalf("Failed to create resource client: %v", err)
				}
				if err = v1alpha1.SchemeBuilder.AddToScheme(r.GetScheme()); err != nil {
					t.Fatalf("Failed to add CloudFoundry scheme: %v", err)
				}

				app := &v1alpha1.App{}
				if err = r.Get(ctx, appName, cfg.Namespace(), app); err != nil {
					t.Fatalf("Failed to get App resource: %v", err)
				}

				externalName := xpmeta.GetExternalName(app)
				if !test.UUIDRegex.MatchString(externalName) {
					t.Fatalf("External name %q does not match expected UUID format after upgrade", externalName)
				}

				preUpgradeExternalName, ok := ctx.Value(appPreUpgradeExternalNameKey{}).(string)
				if !ok {
					t.Fatal("Failed to retrieve pre-upgrade external name from context")
				}

				if externalName != preUpgradeExternalName {
					t.Fatalf("External name changed during upgrade: before=%q, after=%q",
						preUpgradeExternalName, externalName)
				}

				klog.V(4).Info("External name validation passed: format correct and unchanged")
				return ctx
			},
		)

	testenv.Test(t, upgradeTest.Feature())
}
