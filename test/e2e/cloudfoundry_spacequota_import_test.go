//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/e2e-framework/klient/wait"
)

var (
	spaceQuotaImportTestK8sResName  = "e2e-test-space-quota-import"
	spaceQuotaImportTestName        = "e2e-test-space-quota"
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
					OrgRef: &v1.Reference{
						Name: spaceQuotaImportTestOrgName,
						Policy: &v1.Policy{
							Resolution: ptr.To(v1.ResolutionPolicyRequired),
							Resolve:    ptr.To(v1.ResolvePolicyAlways),
						},
					},
					SpacesRefs: []v1.Reference{
						{
							Name: spaceQuotaImportTestSpaceName,
							Policy: &v1.Policy{
								Resolution: ptr.To(v1.ResolutionPolicyRequired),
								Resolve:    ptr.To(v1.ResolvePolicyAlways),
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
		WithDependentResourceDirectory[*v1alpha1.SpaceQuota]("./crs/externalNamesImport"),
	)

	importFeature := importTester.BuildTestFeature("CF SpaceQuota Import Flow").Feature()

	testenv.Test(t, importFeature)

}
