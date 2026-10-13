//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/e2e-framework/klient/wait"
)

var (
	spaceQuotaImportTestK8sResName  = "e2e-test-space-quota-import"
	spaceQuotaImportTestName        = runScopedName("e2e-test-space-quota")
	spaceQuotaImportTestOrgName     = "upgrade-test-org"
	spaceQuotaImportTestSpaceName   = "upgrade-test-import-space"
	SpaceQuotaAllowPaidServicePlans = false
)

func TestSpaceQuotaImportFlow(t *testing.T) {
	importTester := NewImportTester(
		&v1alpha1.SpaceQuota{
			Spec: v1alpha1.SpaceQuotaSpec{
				ForProvider: v1alpha1.SpaceQuotaParameters{
					Name: &spaceQuotaImportTestName,
					OrgRef: &xpv2.Reference{
						Name: spaceQuotaImportTestOrgName,
						Policy: &xpv2.Policy{
							Resolution: ptr.To(xpv2.ResolutionPolicyRequired),
							Resolve:    ptr.To(xpv2.ResolvePolicyAlways),
						},
					},
					SpacesRefs: []xpv2.Reference{
						{
							Name: spaceQuotaImportTestSpaceName,
							Policy: &xpv2.Policy{
								Resolution: ptr.To(xpv2.ResolutionPolicyRequired),
								Resolve:    ptr.To(xpv2.ResolvePolicyAlways),
							},
						},
					},
					AllowPaidServicePlans: &SpaceQuotaAllowPaidServicePlans,
				},
			},
		},
		spaceQuotaImportTestK8sResName,
		WithWaitCreateTimeout[*v1alpha1.SpaceQuota](wait.WithTimeout(5*time.Minute)),
		WithWaitDeletionTimeout[*v1alpha1.SpaceQuota](wait.WithTimeout(5*time.Minute)),
		WithDependentResourceDirectory[*v1alpha1.SpaceQuota](crsDir("externalNamesImport/spaceQuota")),
	)

	importFeature := importTester.BuildTestFeature("CF SpaceQuota Import Flow").Feature()

	testenv.Test(t, importFeature)

}
