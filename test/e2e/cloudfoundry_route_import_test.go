//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"sigs.k8s.io/e2e-framework/klient/wait"
)

var (
	routeImportTestK8sResName = "e2e-test-route-import"
	routeImportTestDomainName = "cfapps.eu12.hana.ondemand.com"
	routeImportTestSpaceName  = "import-test-space-donotdelete"
	routeImportTestOrgName    = "cf-ci-e2e"
	routeImportTestHost       = "route-import-e2e"
)

func TestRouteImportFlow(t *testing.T) {
	importTester := NewImportTester(
		&v1alpha1.Route{
			Spec: v1alpha1.RouteSpec{
				ForProvider: v1alpha1.RouteParameters{
					DomainReference: v1alpha1.DomainReference{
						DomainName: &routeImportTestDomainName,
					},
					SpaceReference: v1alpha1.SpaceReference{
						SpaceName: &routeImportTestSpaceName,
						OrgName:   &routeImportTestOrgName,
					},
					Host: &routeImportTestHost,
				},
			},
		},
		routeImportTestK8sResName,
		WithDependentResourceDirectory[*v1alpha1.Route]("./crs/route"),
		WithWaitCreateTimeout[*v1alpha1.Route](wait.WithTimeout(5*time.Minute)),
		WithWaitDeletionTimeout[*v1alpha1.Route](wait.WithTimeout(5*time.Minute)),
	)

	importFeature := importTester.BuildTestFeature("CF Route Import Flow").Feature()

	testenv.Test(t, importFeature)
}
