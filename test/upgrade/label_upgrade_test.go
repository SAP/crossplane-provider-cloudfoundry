//go:build upgrade

//
// This file (label_upgrade_test.go) contains Test_Label_Migration,
// which validates that default Crossplane labels (crossplane-kind,
// crossplane-name, crossplane-providerconfig) are correctly applied
// to existing resources after a provider upgrade.
//
// Pre-upgrade: Resources created by the old provider version do not have
// default Crossplane labels. Post-upgrade: The new provider reconciles and
// adds the 3 default labels to the external CF resource.

package upgrade

import (
	"context"
	"testing"

	v1alpha1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"k8s.io/klog/v2"
	res "sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

var (
	labelResourceDirectories = []string{
		"./testdata/customCrs/labels/import",
		"./testdata/customCrs/labels/space",
	}
)

func Test_Label_Migration(t *testing.T) {
	const spaceName = "upgrade-test-space"

	upgradeTest := NewCustomUpgradeTest("label-migration-test").
		FromVersion(fromTag).
		ToVersion(toTag).
		WithResourceDirectories(labelResourceDirectories).
		WithCustomPreUpgradeAssessment(
			"Verify no crossplane labels before upgrade",
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				r, err := res.New(cfg.Client().RESTConfig())
				if err != nil {
					t.Fatalf("Failed to create resource client: %v", err)
				}

				err = v1alpha1.SchemeBuilder.AddToScheme(r.GetScheme())
				if err != nil {
					t.Fatalf("Failed to add CloudFoundry scheme: %v", err)
				}

				space := &v1alpha1.Space{}
				err = r.Get(ctx, spaceName, cfg.Namespace(), space)
				if err != nil {
					t.Fatalf("Failed to get Space resource: %v", err)
				}

				// Verify no crossplane-* default labels exist in observation
				labels := space.Status.AtProvider.Labels
				for key := range labels {
					if key == resource.ExternalResourceTagKeyKind ||
						key == resource.ExternalResourceTagKeyName ||
						key == resource.ExternalResourceTagKeyProvider {
						t.Errorf("Pre-upgrade resource unexpectedly has crossplane label: %s", key)
					}
				}

				klog.V(4).Infof("Pre-upgrade label check passed: no crossplane-* labels found on Space %s", spaceName)
				return ctx
			},
		).
		WithCustomPostUpgradeAssessment(
			"Verify default crossplane labels after upgrade",
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				r, err := res.New(cfg.Client().RESTConfig())
				if err != nil {
					t.Fatalf("Failed to create resource client: %v", err)
				}

				err = v1alpha1.SchemeBuilder.AddToScheme(r.GetScheme())
				if err != nil {
					t.Fatalf("Failed to add CloudFoundry scheme: %v", err)
				}

				space := &v1alpha1.Space{}
				err = r.Get(ctx, spaceName, cfg.Namespace(), space)
				if err != nil {
					t.Fatalf("Failed to get Space resource after upgrade: %v", err)
				}

				labels := space.Status.AtProvider.Labels

				// Verify crossplane-kind
				expectedKind := "space.cloudfoundry.crossplane.io"
				if val, ok := labels[resource.ExternalResourceTagKeyKind]; !ok {
					t.Errorf("Missing %s label after upgrade", resource.ExternalResourceTagKeyKind)
				} else if val == nil || *val != expectedKind {
					t.Errorf("Expected %s=%s, got %v", resource.ExternalResourceTagKeyKind, expectedKind, val)
				}

				// Verify crossplane-name
				if val, ok := labels[resource.ExternalResourceTagKeyName]; !ok {
					t.Errorf("Missing %s label after upgrade", resource.ExternalResourceTagKeyName)
				} else if val == nil || *val != spaceName {
					t.Errorf("Expected %s=%s, got %v", resource.ExternalResourceTagKeyName, spaceName, val)
				}

				// Verify crossplane-providerconfig
				if space.GetProviderConfigReference() != nil {
					expectedPC := space.GetProviderConfigReference().Name
					if val, ok := labels[resource.ExternalResourceTagKeyProvider]; !ok {
						t.Errorf("Missing %s label after upgrade", resource.ExternalResourceTagKeyProvider)
					} else if val == nil || *val != expectedPC {
						t.Errorf("Expected %s=%s, got %v", resource.ExternalResourceTagKeyProvider, expectedPC, val)
					}
				}

				klog.V(4).Infof("Post-upgrade label check passed: all 3 crossplane-* labels found on Space %s", spaceName)
				return ctx
			},
		)

	testenv.Test(t, upgradeTest.Feature())
}
