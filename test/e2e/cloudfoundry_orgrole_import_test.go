//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"sigs.k8s.io/e2e-framework/klient/wait"
)

var (
	orgRoleImportTestK8sResName = "e2e-test-org-role-import"
	orgRoleImportTestType       = "User"
	orgRoleImportTestUsername   = "user1@example.com"
	orgRoleImportTestOrgName    = "cf-ci-e2e"
)

func TestOrgRoleImportFlow(t *testing.T) {
	importTester := NewImportTester(
		&v1alpha1.OrgRole{
			Spec: v1alpha1.OrgRoleSpec{
				ForProvider: v1alpha1.OrgRoleParameters{
					OrgReference: v1alpha1.OrgReference{
						OrgName: &orgRoleImportTestOrgName,
					},
					Type:     orgRoleImportTestType,
					Username: orgRoleImportTestUsername,
				},
			},
		},
		orgRoleImportTestK8sResName,
		WithWaitCreateTimeout[*v1alpha1.OrgRole](wait.WithTimeout(5*time.Minute)),
		WithWaitDeletionTimeout[*v1alpha1.OrgRole](wait.WithTimeout(5*time.Minute)),
	)

	importFeature := importTester.BuildTestFeature("CF Org Role Import Flow").Feature()

	testenv.Test(t, importFeature)

}
