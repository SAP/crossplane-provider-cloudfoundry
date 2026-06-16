//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"sigs.k8s.io/e2e-framework/klient/wait"
)

var (
	serviceInstanceImportTestK8sResName = "e2e-test-service-instance-import"
	serviceInstanceImportTestName       = runScopedName("e2e-test-service-instance-import")
	serviceInstanceImportTestSpaceName  = "import-test-space-donotdelete"
	serviceInstanceImportTestOrgName    = testOrgName
	serviceInstanceImportTestRouteURL   = "https://example-service.local/forward"
)

// TestServiceInstanceImportFlow verifies that an existing Cloud Foundry service instance can be
// imported via the external-name annotation: the ImportTester creates a user-provided instance,
// orphans the Kubernetes object (keeping the CF resource), then re-creates the managed resource
// with the captured external-name (GUID) and waits for it to become healthy.
//
// A user-provided instance is used on purpose: it needs neither a service offering/plan nor an
// async provisioning job, which keeps the import flow deterministic in CI.
func TestServiceInstanceImportFlow(t *testing.T) {
	importTester := NewImportTester(
		&v1alpha1.ServiceInstance{
			Spec: v1alpha1.ServiceInstanceSpec{
				ForProvider: v1alpha1.ServiceInstanceParameters{
					Type: v1alpha1.UserProvidedService,
					Name: &serviceInstanceImportTestName,
					SpaceReference: v1alpha1.SpaceReference{
						SpaceName: &serviceInstanceImportTestSpaceName,
						OrgName:   &serviceInstanceImportTestOrgName,
					},
					UserProvided: v1alpha1.UserProvided{
						RouteServiceURL: serviceInstanceImportTestRouteURL,
					},
				},
			},
		},
		serviceInstanceImportTestK8sResName,
		WithWaitCreateTimeout[*v1alpha1.ServiceInstance](wait.WithTimeout(5*time.Minute)),
		WithWaitDeletionTimeout[*v1alpha1.ServiceInstance](wait.WithTimeout(5*time.Minute)),
	)

	importFeature := importTester.BuildTestFeature("CF ServiceInstance Import Flow").Feature()

	testenv.Test(t, importFeature)
}
