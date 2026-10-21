//go:build upgrade

// Test_Label_Migration checks that the Crossplane default tags, written as CF
// labels by the old provider, become CF annotations after the upgrade and the
// legacy labels are removed.

package upgrade

import (
	"context"
	"testing"

	v1alpha1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	res "sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
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
			"Verify crossplane default labels before upgrade",
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

				labels := space.Status.AtProvider.Labels
				if v, ok := labels[resource.ExternalResourceTagKeyName]; !ok || v == nil || *v != spaceName {
					t.Errorf("Pre-upgrade resource expected label %s=%s, got %v", resource.ExternalResourceTagKeyName, spaceName, v)
				}

				klog.V(4).Infof("Pre-upgrade check passed: crossplane-name label found on Space %s", spaceName)
				return ctx
			},
		).
		WithCustomPostUpgradeAssessment(
			"Verify default crossplane annotations after upgrade",
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				r, err := res.New(cfg.Client().RESTConfig())
				if err != nil {
					t.Fatalf("Failed to create resource client: %v", err)
				}

				err = v1alpha1.SchemeBuilder.AddToScheme(r.GetScheme())
				if err != nil {
					t.Fatalf("Failed to add CloudFoundry scheme: %v", err)
				}

				// Synced only proves one reconcile, and AtProvider is refreshed
				// before Update: wait for the next Observe.
				space := &v1alpha1.Space{ObjectMeta: metav1.ObjectMeta{Name: spaceName, Namespace: cfg.Namespace()}}
				err = wait.For(conditions.New(r).ResourceMatch(space, func(obj k8s.Object) bool {
					atProvider := obj.(*v1alpha1.Space).Status.AtProvider
					_, hasAnnotation := atProvider.Annotations[resource.ExternalResourceTagKeyName]
					_, hasLabel := atProvider.Labels[resource.ExternalResourceTagKeyName]
					return hasAnnotation && !hasLabel
				}), wait.WithTimeout(verifyTimeout))
				if err != nil {
					t.Logf("Space %s metadata did not converge after upgrade: %v", spaceName, err)
				}

				err = r.Get(ctx, spaceName, cfg.Namespace(), space)
				if err != nil {
					t.Fatalf("Failed to get Space resource after upgrade: %v", err)
				}

				labels := space.Status.AtProvider.Labels
				annotations := space.Status.AtProvider.Annotations

				expected := map[string]string{
					resource.ExternalResourceTagKeyKind: "space.cloudfoundry.crossplane.io",
					resource.ExternalResourceTagKeyName: spaceName,
				}
				if ref := space.GetProviderConfigReference(); ref != nil {
					expected[resource.ExternalResourceTagKeyProvider] = ref.Name
				}
				for k, want := range expected {
					if val, ok := annotations[k]; !ok || val == nil || *val != want {
						t.Errorf("Expected annotation %s=%s after upgrade, got %v", k, want, val)
					}
				}
				for _, k := range []string{resource.ExternalResourceTagKeyKind, resource.ExternalResourceTagKeyName, resource.ExternalResourceTagKeyProvider} {
					if _, ok := labels[k]; ok {
						t.Errorf("Legacy label %s still present after upgrade", k)
					}
				}

				klog.V(4).Infof("Post-upgrade check passed: default annotations set, legacy labels removed on Space %s", spaceName)
				return ctx
			},
		)

	testenv.Test(t, upgradeTest.Feature())
}
