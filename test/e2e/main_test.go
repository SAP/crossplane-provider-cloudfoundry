//go:build e2e

package e2e

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/crossplane-contrib/xp-testing/pkg/logging"
	"github.com/crossplane-contrib/xp-testing/pkg/setup"
	"github.com/crossplane-contrib/xp-testing/pkg/vendored"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"

	"sigs.k8s.io/e2e-framework/klient/decoder"
	resources "sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/support/kind"
)

var (
	testenv env.Environment
)

// TestMain creates the testing suite for the resource e2e-tests
func TestMain(m *testing.M) {
	var verbosity = 4
	logging.EnableVerboseLogging(&verbosity)

	namespace := envconf.RandomName("test-ns", 16)

	SetupClusterWithCrossplane(namespace)

	os.Exit(testenv.Run(m))
}

func SetupClusterWithCrossplane(namespace string) {
	testenv = env.New()

	secretData := getProviderConfigSecretData()
	secretName := "cf-provider-secret"

	clusterCredentials := setup.ProviderCredentials{
		SecretData: secretData,
		SecretName: &secretName,
	}

	deploymentRuntimeConfig := vendored.DeploymentRuntimeConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cf-provider-runtime-config",
		},
		Spec: vendored.DeploymentRuntimeConfigSpec{
			DeploymentTemplate: &vendored.DeploymentTemplate{
				Spec: &appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{},
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "package-runtime",
									Args: []string{"--debug", "--sync=10s"},
								},
							},
						},
					},
				},
			},
		},
	}

	clusterSetup := setup.ClusterSetup{
		ProviderName:            "provider-cloudfoundry",
		ProviderCredential:      &clusterCredentials,
		CrossplaneSetup:         setup.CrossplaneSetup{Version: "1.20.1", Registry: setup.DockerRegistry},
		DeploymentRuntimeConfig: &deploymentRuntimeConfig,
	}

	clusterSetup.Configure(testenv, &kind.Cluster{})

	testenv.Setup(
		envfuncs.CreateNamespace(namespace),
		func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
			cfg.WithNamespace(namespace)
			return ctx, nil
		},
		func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
			r, _ := resources.New(cfg.Client().RESTConfig())
			err := decoder.DecodeEachFile(
				ctx, os.DirFS("./provider"), "*",
				decoder.CreateHandler(r),
				decoder.MutateNamespace(namespace),
			)
			if err != nil && !strings.Contains(err.Error(), "already exists") {
				klog.Error("Error creating ProviderConfig:", "err", err)
			}
			return ctx, nil
		},
	)
}
