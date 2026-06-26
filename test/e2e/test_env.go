//go:build e2e

package e2e

import (
	"github.com/crossplane-contrib/xp-testing/pkg/envvar"
)

var (
	UUT_IMAGES_KEY     = "UUT_IMAGES"
	UUT_CONFIG_KEY     = "package"
	UUT_CONTROLLER_KEY = "controller"
	ENDPOINT_KEY       = "apiEndpoint"
	CREDENTIALS_KEY    = "credentials"
)

func getProviderConfigSecretData() map[string]string {
	secretData := map[string]string{
		CREDENTIALS_KEY: envvar.GetOrPanic("CF_CREDENTIALS"),
		ENDPOINT_KEY:    envvar.GetOrPanic("CF_ENVIRONMENT"),
	}
	return secretData
}
