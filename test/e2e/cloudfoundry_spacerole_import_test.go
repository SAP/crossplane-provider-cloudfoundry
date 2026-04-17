//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"sigs.k8s.io/e2e-framework/klient/wait"
)

var (
	spaceRoleImportTestK8sResName = "e2e-test-space-role-import"
	spaceRoleImportTestType       = "Developer"
	spaceRoleImportTestUsername   = "user1@example.com"
	spaceRoleImportTestOrgName    = "cf-ci-e2e"
	spaceRoleImportTestSpaceName  = "import-test-space-donotdelete"
)

func TestSpaceRoleImportFlow(t *testing.T) {
	importTester := NewImportTester(
		&v1alpha1.SpaceRole{
			Spec: v1alpha1.SpaceRoleSpec{
				ForProvider: v1alpha1.SpaceRoleParameters{
					SpaceReference: v1alpha1.SpaceReference{
						SpaceName: &spaceRoleImportTestSpaceName,
						OrgName:   &spaceRoleImportTestOrgName,
					},
					Type:     spaceRoleImportTestType,
					Username: spaceRoleImportTestUsername,
				},
			},
		},
		spaceRoleImportTestK8sResName,
		WithWaitCreateTimeout[*v1alpha1.SpaceRole](wait.WithTimeout(5*time.Minute)),
		WithWaitDeletionTimeout[*v1alpha1.SpaceRole](wait.WithTimeout(5*time.Minute)),
	)

	importFeature := importTester.BuildTestFeature("CF Space Role Import Flow").Feature()

	testenv.Test(t, importFeature)

}
