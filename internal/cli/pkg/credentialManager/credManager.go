package credentialManager

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/spf13/viper"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/v1beta1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/cli/adapters"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/crossplaneimport/erratt"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/crossplaneimport/provider"
)

func RetrieveCredentials(ctx context.Context, kubeClient client.Client) (provider.Credentials, error) {
	slog.Debug("fetching ProviderConfig",
		"providerconfig.name",
		viper.GetString("providerconfig.name"))
	providerConfig := v1beta1.ProviderConfig{}
	err := kubeClient.Get(ctx, client.ObjectKey{
		Name: viper.GetString("providerconfig.name"),
	}, &providerConfig)
	if err != nil {
		return nil, erratt.Wrap("error getting ProviderConfig resource",
			slog.String("providerconfig.name", viper.GetString("providerconfig.name")),
			slog.Any("error", err),
		)
	}
	slog.Debug("obtaining Cloud Foundry credentials via ProviderConfig",
		"providerconfig.name",
		viper.GetString("providerconfig.name"),
		"credentials-source",
		providerConfig.Spec.Credentials.Source,
	)
	secret, err := resource.CommonCredentialExtractor(ctx, providerConfig.Spec.Credentials.Source, kubeClient, providerConfig.Spec.Credentials.CommonCredentialSelectors)
	if err != nil {
		return nil, erratt.Wrap("error getting secrets of ProviderConfig resource",
			slog.String("providerconfig.name", viper.GetString("providerconfig.name")),
			slog.String("credentials-source", string(providerConfig.Spec.Credentials.Source)),
			slog.Any("err", err))
	}

	type cfSecret struct {
		User     string `json:"user"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	cfs := cfSecret{}
	err = json.Unmarshal(secret, &cfs)
	if err != nil {
		slog.Error("error unmarshalling secret json",
			"secret",
			string(secret))
		return nil, err
	}
	if providerConfig.Spec.APIEndpoint == nil {
		return nil, erratt.Wrap("APIEndpoint is not set in providerConfig",
			slog.String("providerconfig.name", viper.GetString("providerconfig.name")),
		)
	}
	creds := &adapters.CFCredentials{
		ApiEndpoint: *providerConfig.Spec.APIEndpoint,
		Email:       cfs.Email,
		Password:    cfs.Password,
	}
	return creds, err
}
