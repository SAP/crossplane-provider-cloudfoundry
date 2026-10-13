//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
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
						RouteRef: &xpv2.Reference{
							Name: srbImportTestRouteName,
							Policy: &xpv2.Policy{
								Resolution: ptr.To(xpv2.ResolutionPolicyRequired),
								Resolve:    ptr.To(xpv2.ResolvePolicyAlways),
							},
						},
					},
					ServiceInstanceReference: v1alpha1.ServiceInstanceReference{
						ServiceInstanceRef: &xpv2.Reference{
							Name: srbImportTestServiceName,
							Policy: &xpv2.Policy{
								Resolution: ptr.To(xpv2.ResolutionPolicyRequired),
								Resolve:    ptr.To(xpv2.ResolvePolicyAlways),
							},
						},
					},
				},
			},
		},
		srbImportTestK8sResName,
		WithWaitCreateTimeout[*v1alpha1.ServiceRouteBinding](wait.WithTimeout(5*time.Minute)),
		WithWaitDeletionTimeout[*v1alpha1.ServiceRouteBinding](wait.WithTimeout(5*time.Minute)),
		WithDependentResourceDirectory[*v1alpha1.ServiceRouteBinding](crsDir("externalNamesImport/serviceRouteBinding")),
	)

	importFeature := importTester.BuildTestFeature("CF ServiceRouteBinding Import Flow").Feature()

	testenv.Test(t, importFeature)

}
