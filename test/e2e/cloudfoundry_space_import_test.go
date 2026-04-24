//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"sigs.k8s.io/e2e-framework/klient/wait"
)

var (
	spaceImportTestK8sResName = "e2e-test-space-import"
	SpaceImportTestSpaceName  = "e2e-test-space-import"
	spaceImportTestOrgName    = "cf-ci-e2e"
)

func TestSpaceImportFlow(t *testing.T) {
	importTester := NewImportTester(
		&v1alpha1.Space{
			Spec: v1alpha1.SpaceSpec{
				ForProvider: v1alpha1.SpaceParameters{
					Name: SpaceImportTestSpaceName,
					OrgReference: v1alpha1.OrgReference{
						OrgName: &spaceImportTestOrgName,
					},
				},
			},
		},
		spaceImportTestK8sResName,
		WithWaitCreateTimeout[*v1alpha1.Space](wait.WithTimeout(5*time.Minute)),
		WithWaitDeletionTimeout[*v1alpha1.Space](wait.WithTimeout(5*time.Minute)),
	)

	importFeature := importTester.BuildTestFeature("CF Space Import Flow").Feature()

	testenv.Test(t, importFeature)

}
