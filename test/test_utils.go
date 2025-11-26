package test

import (
	"context"
	"encoding/json"
	"fmt"

	cloudfoundryv1beta1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/v1beta1"
	"github.com/crossplane-contrib/xp-testing/pkg/envvar"
	"github.com/crossplane-contrib/xp-testing/pkg/logging"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

const (
	crossplaneSystemNamespace = "crossplane-system"
)

// CFCredentials represents the CloudFoundry credentials structure
type CFCredentials struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// SetupLogging configures klog and controller-runtime logging with the specified verbosity
// TODO: Consider extracting to shared package - identical in BTP and CF providers
func SetupLogging(verbosity int) {
	logging.EnableVerboseLogging(&verbosity)
	zl := zap.New(zap.UseDevMode(true))
	ctrl.SetLogger(zl)
}

// GetCFCredentialsOrPanic retrieves CloudFoundry credentials from environment variables
// and returns them as a map suitable for creating a Kubernetes secret
//
// Required environment variables:
//   - CF_EMAIL: Email address for CF authentication
//   - CF_USERNAME: Username for CF authentication
//   - CF_PASSWORD: Password for CF authentication
//
// Returns: map with "credentials" key containing JSON-encoded credentials
func GetCFCredentialsOrPanic() map[string][]byte {
	email := envvar.GetOrPanic("CF_EMAIL")
	username := envvar.GetOrPanic("CF_USERNAME")
	password := envvar.GetOrPanic("CF_PASSWORD")

	creds := CFCredentials{
		Email:    email,
		Username: username,
		Password: password,
	}

	credsJSON, err := json.Marshal(creds)
	if err != nil {
		panic(fmt.Errorf("failed to marshal CF credentials: %w", err))
	}

	return map[string][]byte{
		"credentials": credsJSON,
	}
}

// ApplySecretInCrossplaneNamespace creates a secret in the crossplane-system namespace
// This is used to store CloudFoundry credentials that the provider will use
//
// Parameters:
//   - secretName: Name of the secret to create
//   - data: Map of secret data (key -> value as bytes)
//
// Returns: An env.Func that can be used with testenv.Setup()
func ApplySecretInCrossplaneNamespace(secretName string, data map[string][]byte) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: crossplaneSystemNamespace,
			},
			Data: data,
		}

		client := cfg.Client()
		if err := client.Resources().Create(ctx, secret); err != nil {
			return ctx, fmt.Errorf("failed to create secret %s in namespace %s: %w",
				secretName, crossplaneSystemNamespace, err)
		}

		klog.V(4).Infof("created secret %s in namespace %s", secretName, crossplaneSystemNamespace)
		return ctx, nil
	}
}

// CreateProviderConfigFn creates a CloudFoundry ProviderConfig resource
// This configures the provider to connect to a specific CloudFoundry instance
//
// Parameters:
//   - namespace: Namespace for the test (currently unused but kept for consistency)
//   - cfEndpoint: CloudFoundry API endpoint URL (e.g., "https://api.cf.eu12.hana.ondemand.com")
//   - secretName: Name of the secret containing CF credentials
//
// Returns: An env.Func that can be used with testenv.Setup()
//
// Note: The ProviderConfig is named "default" which is the default name that
// CloudFoundry managed resources will use if no specific providerConfigRef is set
func CreateProviderConfigFn(namespace, cfEndpoint, secretName string) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		providerConfig := &cloudfoundryv1beta1.ProviderConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "default",
			},
			Spec: cloudfoundryv1beta1.ProviderConfigSpec{
				APIEndpoint: &cfEndpoint,
				Credentials: cloudfoundryv1beta1.ProviderCredentials{
					Source: "Secret",
					CommonCredentialSelectors: xpv1.CommonCredentialSelectors{
						SecretRef: &xpv1.SecretKeySelector{
							SecretReference: xpv1.SecretReference{
								Name:      secretName,
								Namespace: crossplaneSystemNamespace,
							},
							Key: "credentials",
						},
					},
				},
			},
		}

		client := cfg.Client()
		if err := client.Resources().Create(ctx, providerConfig); err != nil {
			return ctx, fmt.Errorf("failed to create ProviderConfig 'default': %w", err)
		}

		klog.V(4).Infof("created ProviderConfig 'default' for CF endpoint %s", cfEndpoint)
		return ctx, nil
	}
}
