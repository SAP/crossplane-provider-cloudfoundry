//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/e2e-framework/klient/wait"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

var (
	domainImportTestK8sResName = "e2e-test-domain-import"
	domainImportTestName       = runScopedName("e2e-test-domain-import") + ".eu12.hana.ondemand.com"
	domainImportTestOrgName    = "upgrade-test-org"
)

func TestDomainImportFlow(t *testing.T) {
	importTester := NewImportTester(
		&v1alpha1.Domain{
			Spec: v1alpha1.DomainSpec{
				ForProvider: v1alpha1.DomainParameters{
					Name: domainImportTestName,
					OrgReference: v1alpha1.OrgReference{
						OrgRef: &xpv2.Reference{
							Name: domainImportTestOrgName,
							Policy: &xpv2.Policy{
								Resolution: ptr.To(xpv2.ResolutionPolicyRequired),
								Resolve:    ptr.To(xpv2.ResolvePolicyAlways),
							},
						},
					},
				},
			},
		},
		domainImportTestK8sResName,
		WithWaitCreateTimeout[*v1alpha1.Domain](wait.WithTimeout(5*time.Minute)),
		WithWaitDeletionTimeout[*v1alpha1.Domain](wait.WithTimeout(5*time.Minute)),
		WithDependentResourceDirectory[*v1alpha1.Domain](crsDir("externalNamesImport/domain")),
	)

	importFeature := importTester.BuildTestFeature("CF Domain Import Flow").Feature()

	testenv.Test(t, importFeature)
}
