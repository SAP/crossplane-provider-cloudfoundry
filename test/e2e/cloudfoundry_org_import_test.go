//go:build e2e

package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

var (
	orgImportTestK8sResName = "e2e-test-org-import"
	orgImportTestOrgName    = testOrgName
)

func TestOrgImportFlow(t *testing.T) {
	importTester := NewImportTester(
		&v1alpha1.Organization{
			Spec: v1alpha1.OrgSpec{
				ForProvider: v1alpha1.OrgParameters{
					Name: orgImportTestOrgName,
				},
			},
		},
		orgImportTestK8sResName,
		WithDependentResourceDirectory[*v1alpha1.Organization](crsDir("org")),
		WithWaitCreateTimeout[*v1alpha1.Organization](wait.WithTimeout(5*time.Minute)),
		WithWaitDeletionTimeout[*v1alpha1.Organization](wait.WithTimeout(5*time.Minute)),
		WithCustomTeardown(func(it *ImportTester[*v1alpha1.Organization], ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			resource := it.BaseResource.DeepCopyObject().(*v1alpha1.Organization)
			MustGetResource(t, cfg, it.GetPrefixedName(), nil, resource)

			// Switch to observe-only so teardown does not delete the shared external resource.
			resource.SetManagementPolicies(xpv1.ManagementPolicies{xpv1.ManagementActionObserve})
			if err := cfg.Client().Resources().Update(ctx, resource); err != nil {
				t.Fatalf("Failed to switch imported resource to observe-only before teardown: %v", err)
			}

			log("Deleting imported resource", resource, func() {
				AwaitResourceDeletionOrFail(ctx, t, cfg, resource, it.WaitDeletionTimeout)
			})

			log("Deleting dependent resources", resource, func() {
				if it.DependentResourceDirectory != "" {
					DeleteResourcesIgnoreMissing(ctx, t, cfg, it.DependentResourceDirectory, it.WaitDeletionTimeout)
				}
			})

			return ctx
		}),
	)

	importFeature := importTester.BuildTestFeature("CF Org Import Flow").Feature()

	testenv.Test(t, importFeature)
}
