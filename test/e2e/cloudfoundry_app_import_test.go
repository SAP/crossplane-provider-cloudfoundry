//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"sigs.k8s.io/e2e-framework/klient/wait"
)

var (
	appImportTestK8sResName = "e2e-test-app-import"
	appImportTestAppName    = "e2e-test-app-import"
	appImportTestSpace      = "space-donotdelete"
	appImportTestOrg        = testOrgName
)

func TestAppImportFlow(t *testing.T) {
	importTester := NewImportTester(
		&v1alpha1.App{
			Spec: v1alpha1.AppSpec{
				ForProvider: v1alpha1.AppParameters{
					Name:      appImportTestAppName,
					Lifecycle: "docker",
					SpaceReference: v1alpha1.SpaceReference{
						SpaceName: &appImportTestSpace,
						OrgName:   &appImportTestOrg,
					},
					Docker: &v1alpha1.DockerConfiguration{
						Image: "loud/hello_co:latest",
					},
				},
			},
		},
		appImportTestK8sResName,
		WithWaitCreateTimeout[*v1alpha1.App](wait.WithTimeout(10*time.Minute)),
		WithWaitDeletionTimeout[*v1alpha1.App](wait.WithTimeout(5*time.Minute)),
	)

	importFeature := importTester.BuildTestFeature("CF App Import Flow").Feature()

	testenv.Test(t, importFeature)
}
