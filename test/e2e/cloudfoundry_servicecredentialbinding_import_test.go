//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	v1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
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
					ServiceInstanceRef: &v1.Reference{
						Name: scbImportTestServiceInstRef,
						Policy: &v1.Policy{
							Resolution: ptr.To(v1.ResolutionPolicyRequired),
							Resolve:    ptr.To(v1.ResolvePolicyAlways),
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
