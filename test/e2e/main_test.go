//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/crossplane-contrib/xp-testing/pkg/envvar"
	"github.com/crossplane-contrib/xp-testing/pkg/images"
	"github.com/crossplane-contrib/xp-testing/pkg/logging"
	"github.com/crossplane-contrib/xp-testing/pkg/setup"
	"github.com/crossplane-contrib/xp-testing/pkg/vendored"
	"k8s.io/klog"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/config"
	"github.com/pkg/errors"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients"
	"sigs.k8s.io/e2e-framework/klient/decoder"
	resources "sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/support/kind"

	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

var testenv env.Environment
var testOrgName = "cf-ci-e2e"

var (
	UUT_CONFIG_KEY     = "package"
	UUT_CONTROLLER_KEY = "controller"
	ENDPOINT_KEY       = "apiEndpoint"
	CREDENTIALS_KEY    = "credentials"
)

// TestMain creates the testing suite for the resource e2e-tests
func TestMain(m *testing.M) {
	var verbosity = 4
	logging.EnableVerboseLogging(&verbosity)
	testenv = env.New()

	namespace := envconf.RandomName("test-ns", 16)

	imgs := images.GetImagesFromEnvironmentOrPanic(UUT_CONFIG_KEY, &UUT_CONTROLLER_KEY)

	secretData := getProviderConfigSecretData()
	secretName := "cf-provider-secret"

	clusterCredentials := setup.ProviderCredentials{
		SecretData: secretData,
		SecretName: &secretName,
	}
	// Enhance interface for one- based providers
	clusterSetup := setup.ClusterSetup{
		ProviderName:       "provider-cloudfoundry",
		Images:             imgs,
		ProviderCredential: &clusterCredentials,
		CrossplaneSetup:    setup.CrossplaneSetup{Version: "1.14.3"},
		ControllerConfig: &vendored.ControllerConfig{
			Spec: vendored.ControllerConfigSpec{
				Image: imgs.ControllerImage,
			},
		},
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

			if errdecode != nil {
				klog.Error("Error Details", "errdecode", errdecode)
			}

			// propagate context value
			return ctx, nil
		},
	)

	os.Exit(testenv.Run(m))
}

func getProviderConfigSecretData() map[string]string {
	secretData := map[string]string{
		CREDENTIALS_KEY: envvar.GetOrPanic("CF_CREDENTIALS"),
		ENDPOINT_KEY:    envvar.GetOrPanic("CF_ENVIRONMENT"),
	}
	return secretData

}

func getCfClient() (*client.Client, error) {
	secretData := getProviderConfigSecretData()

	endpoint := secretData[ENDPOINT_KEY]
	creds := secretData[CREDENTIALS_KEY]

	var s clients.CfCredentials
	if err := json.Unmarshal([]byte(creds), &s); err != nil {
		return nil, errors.Wrap(err, "cannot extract cloud foundry credentials from env variable")
	}
	cfg, err := config.New(endpoint, config.UserPassword(s.Email, s.Password), config.SkipTLSValidation())
	if err != nil {
		return nil, errors.Wrap(err, "cannot configure cloudfoundry client")
	}

	return client.New(cfg)
}
