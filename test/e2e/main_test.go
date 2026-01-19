//go:build e2e

package e2e

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/crossplane-contrib/xp-testing/pkg/logging"
	"github.com/crossplane-contrib/xp-testing/pkg/setup"
	"k8s.io/klog"

	"sigs.k8s.io/e2e-framework/klient/decoder"
	resources "sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/support/kind"

	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

// TestMain creates the testing suite for the resource e2e-tests
func TestMain(m *testing.M) {
	var verbosity = 4
	logging.EnableVerboseLogging(&verbosity)
	testenv = env.New()

	namespace := envconf.RandomName("test-ns", 16)

	secretData := getProviderConfigSecretData()
	secretName := "cf-provider-secret"

	clusterCredentials := setup.ProviderCredentials{
		SecretData: secretData,
		SecretName: &secretName,
	}
	// Enhance interface for one- based providers
	clusterSetup := setup.ClusterSetup{
		ProviderName:       "provider-cloudfoundry",
		ProviderCredential: &clusterCredentials,
		CrossplaneSetup:    setup.CrossplaneSetup{Version: "1.16.0"},
	}

	_ = clusterSetup.Configure(testenv, &kind.Cluster{})

	testenv.BeforeEachTest(
		func(ctx context.Context, cfg *envconf.Config, t *testing.T) (context.Context, error) {
			r, _ := resources.New(cfg.Client().RESTConfig())

			errdecode := decoder.DecodeEachFile(
				ctx, os.DirFS("./provider"), "*",
				decoder.CreateHandler(r),
				decoder.MutateNamespace(namespace),
			)

			resetTestOrg(ctx, t)

			if errdecode != nil && !strings.Contains(errdecode.Error(), "already exists") {
				klog.Error("Error Details:", "errdecode", errdecode)
			}
			// propagate context value
			return ctx, nil
		},
	)

	os.Exit(testenv.Run(m))
}
