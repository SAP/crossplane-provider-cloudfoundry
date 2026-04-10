//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"sigs.k8s.io/e2e-framework/klient/wait"
)

var (
	spaceRoleImportK8sResName = "e2e-test-space-role-import"
	spaceRoleTypeE2e          = "Developer"
	spaceRoleUsernameE2e      = "user1@example.com"
)

func TestSpaceRoleImportFlow(t *testing.T) {
	importTester := NewImportTester(
		&v1alpha1.SpaceRole{
			Spec: v1alpha1.SpaceRoleSpec{
				ForProvider: v1alpha1.SpaceRoleParameters{
					SpaceReference: v1alpha1.SpaceReference{
						SpaceName: &testSpaceName,
						OrgName:   &testOrgName,
					},
					Type:     spaceRoleTypeE2e,
					Username: spaceRoleUsernameE2e,
				},
			},
		},
		spaceRoleImportK8sResName,
		WithWaitCreateTimeout[*v1alpha1.SpaceRole](wait.WithTimeout(5*time.Minute)),
		WithWaitDeletionTimeout[*v1alpha1.SpaceRole](wait.WithTimeout(5*time.Minute)),
	)

	importFeature := importTester.BuildTestFeature("CF Space Role Import Flow").Feature()

	testenv.Test(t, importFeature)

}
