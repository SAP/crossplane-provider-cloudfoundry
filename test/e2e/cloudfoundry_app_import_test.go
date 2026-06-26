//go:build e2e

package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients"
	xpmeta "github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

var (
	appImportTestK8sResName = "e2e-test-app-import"
	appImportTestAppName    = runScopedName("e2e-test-app-import")
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
					Processes: []v1alpha1.ProcessConfiguration{
						{
							Type: ptr.To("web"),
							HealthCheckConfiguration: v1alpha1.HealthCheckConfiguration{
								HealthCheckType:         ptr.To("http"),
								HealthCheckHTTPEndpoint: ptr.To("/"),
							},
						},
					},
				},
			},
		},
		appImportTestK8sResName,
		WithWaitCreateTimeout[*v1alpha1.App](wait.WithTimeout(10*time.Minute)),
		WithWaitDeletionTimeout[*v1alpha1.App](wait.WithTimeout(5*time.Minute)),
	)

	importFeature := importTester.BuildTestFeature("CF App Import Flow").
		Assess("External-name is a valid GUID after import", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			resource := &v1alpha1.App{}
			MustGetResource(t, cfg, importTester.GetPrefixedName(), nil, resource)
			externalName := xpmeta.GetExternalName(resource)
			if !clients.IsValidGUID(externalName) {
				t.Errorf("expected GUID external-name after import, got %q", externalName)
			}
			return ctx
		}).
		Feature()

	testenv.Test(t, importFeature)
}
