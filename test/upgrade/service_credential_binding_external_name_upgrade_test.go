//go:build upgrade

//
// This file contains Test_ServiceCredentialBinding_External_Name,
// which validates that ServiceCredentialBinding resources maintain proper external-name formatting
// during provider upgrades. Specifically, it verifies:
//   - External-name annotation exists and follows UUID format
//   - External-name value remains unchanged after provider upgrade
//

package upgrade

import (
	"context"
	"testing"

	v1alpha1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/test"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"k8s.io/klog/v2"
	res "sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

// preUpgradeExternalNameKey is an unexported context key for storing pre-upgrade external name
type preUpgradeExternalNameKey struct{}

var (
	serviceCredentialBindingCustomResourceDirectories = []string{
		"./testdata/customCrs/externalNames/import",
		"./testdata/customCrs/externalNames/serviceCredentialBinding",
	}
)

func Test_ServiceCredentialBinding_External_Name(t *testing.T) {
	const serviceCredentialBindingName = "upgrade-test-external-name-service-credential-binding"

	upgradeTest := NewCustomUpgradeTest("service-credential-binding-external-name-test").
		FromVersion(fromTag).
		ToVersion(toTag).
		WithResourceDirectories(serviceCredentialBindingCustomResourceDirectories).
		WithCustomPreUpgradeAssessment(
			"Verify external name before upgrade",
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				r, err := res.New(cfg.Client().RESTConfig())
				if err != nil {
					t.Fatalf("Failed to create resource client: %v", err)
				}

				err = v1alpha1.SchemeBuilder.AddToScheme(r.GetScheme())
				if err != nil {
					t.Fatalf("Failed to add CloudFoundry scheme: %v", err)
				}

				serviceCredentialBinding := &v1alpha1.ServiceCredentialBinding{}

				err = r.Get(ctx, serviceCredentialBindingName, cfg.Namespace(), serviceCredentialBinding)
				if err != nil {
					t.Fatalf("Failed to get ServiceCredentialBinding resource: %v", err)
				}

				externalName := meta.GetExternalName(serviceCredentialBinding)
				if externalName == "" {
					t.Fatal("External name annotation does not exist")
				}

				klog.V(4).Infof("Pre-upgrade external name: %s", externalName)

				if !test.UUIDRegex.MatchString(externalName) {
					t.Fatalf("External name '%s' does not match expected UUID format", externalName)
				}

				return context.WithValue(ctx, preUpgradeExternalNameKey{}, externalName)
			},
		).
		WithCustomPostUpgradeAssessment(
			"Verify external name after upgrade",
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				serviceCredentialBinding := &v1alpha1.ServiceCredentialBinding{}
				r, err := res.New(cfg.Client().RESTConfig())
				if err != nil {
					t.Fatalf("Failed to create resource client: %v", err)
				}

				err = v1alpha1.SchemeBuilder.AddToScheme(r.GetScheme())
				if err != nil {
					t.Fatalf("Failed to add CloudFoundry scheme: %v", err)
				}

				err = r.Get(ctx, serviceCredentialBindingName, cfg.Namespace(), serviceCredentialBinding)
				if err != nil {
					t.Fatalf("Failed to get ServiceCredentialBinding resource: %v", err)
				}

				externalName := meta.GetExternalName(serviceCredentialBinding)
				if externalName == "" {
					t.Fatal("External name annotation does not exist after upgrade")
				}

				klog.V(4).Infof("Post-upgrade external name: %s", externalName)

				if !test.UUIDRegex.MatchString(externalName) {
					t.Fatalf("External name '%s' does not match expected UUID format after upgrade", externalName)
				}

				preUpgradeExternalName, ok := ctx.Value(preUpgradeExternalNameKey{}).(string)
				if !ok {
					t.Fatal("Failed to retrieve pre-upgrade external name from context")
				}

				if externalName != preUpgradeExternalName {
					t.Fatalf("External name changed during upgrade: before='%s', after='%s'",
						preUpgradeExternalName, externalName)
				}

				klog.V(4).Info("External name validation passed: format correct and unchanged")
				return ctx
			},
		)

	testenv.Test(t, upgradeTest.Feature())
}
