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
	srbImportTestK8sResName  = "e2e-test-service-route-binding-import"
	srbImportTestRouteName   = "upgrade-test-route"
	srbImportTestServiceName = "upgrade-test-serviceinstance"
)

func TestServiceRouteBindingImportFlow(t *testing.T) {
	importTester := NewImportTester(
		&v1alpha1.ServiceRouteBinding{
			Spec: v1alpha1.ServiceRouteBindingSpec{
				ForProvider: v1alpha1.ServiceRouteBindingParameters{
					RouteReference: v1alpha1.RouteReference{
						RouteRef: &v1.Reference{
							Name: srbImportTestRouteName,
							Policy: &v1.Policy{
								Resolution: ptr.To(v1.ResolutionPolicyRequired),
								Resolve:    ptr.To(v1.ResolvePolicyAlways),
							},
						},
					},
					ServiceInstanceReference: v1alpha1.ServiceInstanceReference{
						ServiceInstanceRef: &v1.Reference{
							Name: srbImportTestServiceName,
							Policy: &v1.Policy{
								Resolution: ptr.To(v1.ResolutionPolicyRequired),
								Resolve:    ptr.To(v1.ResolvePolicyAlways),
							},
						},
					},
				},
			},
		},
		srbImportTestK8sResName,
		WithWaitCreateTimeout[*v1alpha1.ServiceRouteBinding](wait.WithTimeout(5*time.Minute)),
		WithWaitDeletionTimeout[*v1alpha1.ServiceRouteBinding](wait.WithTimeout(5*time.Minute)),
		WithDependentResourceDirectory[*v1alpha1.ServiceRouteBinding]("./crs/externalNamesImport/serviceRouteBinding"),
	)

	importFeature := importTester.BuildTestFeature("CF ServiceRouteBinding Import Flow").Feature()

	testenv.Test(t, importFeature)

}
