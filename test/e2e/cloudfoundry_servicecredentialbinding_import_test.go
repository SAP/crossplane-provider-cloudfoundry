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
	scbImportTestK8sResName     = "e2e-test-scb-import"
	scbImportTestName           = "e2e-test-scb-import"
	scbImportTestServiceInstRef = "import-test-scb-serviceinstance"
)

func TestServiceCredentialBindingImportFlow(t *testing.T) {
	importTester := NewImportTester(
		&v1alpha1.ServiceCredentialBinding{
			Spec: v1alpha1.ServiceCredentialBindingSpec{
				ForProvider: v1alpha1.ServiceCredentialBindingParameters{
					Type: "key",
					Name: &scbImportTestName,
					ServiceInstanceRef: &xpv2.Reference{
						Name: scbImportTestServiceInstRef,
						Policy: &xpv2.Policy{
							Resolution: ptr.To(xpv2.ResolutionPolicyRequired),
							Resolve:    ptr.To(xpv2.ResolvePolicyAlways),
						},
					},
				},
			},
		},
		scbImportTestK8sResName,
		WithWaitCreateTimeout[*v1alpha1.ServiceCredentialBinding](wait.WithTimeout(5*time.Minute)),
		WithWaitDeletionTimeout[*v1alpha1.ServiceCredentialBinding](wait.WithTimeout(5*time.Minute)),
		WithDependentResourceDirectory[*v1alpha1.ServiceCredentialBinding]("./crs/externalNamesImport/serviceCredentialBinding"),
	)

	importFeature := importTester.BuildTestFeature("CF ServiceCredentialBinding Import Flow").Feature()

	testenv.Test(t, importFeature)
}
