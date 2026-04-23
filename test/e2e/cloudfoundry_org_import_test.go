//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"sigs.k8s.io/e2e-framework/klient/wait"
)

var (
	orgImportTestK8sResName = "e2e-test-org-import"
	orgImportTestOrgName    = "cf-ci-e2e"
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
		WithDependentResourceDirectory[*v1alpha1.Organization]("./crs/org"),
		WithWaitCreateTimeout[*v1alpha1.Organization](wait.WithTimeout(5*time.Minute)),
		WithWaitDeletionTimeout[*v1alpha1.Organization](wait.WithTimeout(5*time.Minute)),
	)

	importFeature := importTester.BuildTestFeature("CF Org Import Flow").Feature()

	testenv.Test(t, importFeature)
}
