/*
Copyright 2023 SAP SE
*/

package clients

import (
	"context"
	"encoding/json"

	cfv3 "github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/config"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/v1beta1"
)

// CfCredentials used to authenticate with the provider
// FIXME: not consistent with other providers.
type CfCredentials struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
	Passcode string `json:"passcode"`
	Origin   string `json:"origin,omitempty"`
}

const (
	// error messages
	errNoProviderConfig     = "no providerConfigRef provided"
	errGetProviderConfig    = "cannot get referenced ProviderConfig"
	errTrackUsage           = "cannot track ProviderConfig usage"
	errExtractCredentials   = "cannot extract credentials"
	errExtractEndpoint      = "cannot extract endpoint"
	errUnmarshalCredentials = "cannot unmarshal cloudfoundry credentials as JSON"
	errUnmarshalEndpoint    = "cannot unmarshal cloudfoundry endpoint as JSON"
	errNoEndpoint           = "no API endpoint is configured in ProviderConfig"
)

// GetCredentialConfig returns a config.Config for the given managed resource
func GetCredentialConfig(ctx context.Context, client client.Client, mg resource.Managed) (*config.Config, error) {
	pc, err := getProviderConfig(ctx, client, mg)
	if err != nil {
		return nil, errors.Wrap(err, errGetProviderConfig)
	}
	cred, err := getCredentials(ctx, client, pc)
	if err != nil {
		return nil, errors.Wrap(err, errExtractCredentials)
	}

	url, err := getEndpoint(ctx, client, pc)
	if err != nil {
		return nil, errors.Wrap(err, errExtractEndpoint)
	}

	opts := []config.Option{
		config.UserPassword(cred.Email, cred.Password),
		config.SkipTLSValidation(),
	}
	if cred.Origin != "" {
		opts = append(opts, config.Origin(cred.Origin))
	}
	return config.New(*url, opts...)
}

func getProviderConfig(ctx context.Context, client client.Client, mg resource.Managed) (*v1beta1.ProviderConfig, error) {
	mm, ok := mg.(resource.ModernManaged)
	if !ok {
		return nil, errors.New(errNoProviderConfig)
	}
	ref := mm.GetProviderConfigReference()
	if ref == nil {
		return nil, errors.New(errNoProviderConfig)
	}
	pc := &v1beta1.ProviderConfig{}
	if err := client.Get(ctx, types.NamespacedName{
		Name:      ref.Name,
		Namespace: mg.GetNamespace(),
	}, pc); err != nil {
		return nil, err
	}
	return pc, nil
}

func getCredentials(ctx context.Context, client client.Client, pc *v1beta1.ProviderConfig) (*CfCredentials, error) {
	buf, err := resource.CommonCredentialExtractor(ctx, pc.Spec.Credentials.Source, client, pc.Spec.Credentials.CommonCredentialSelectors)
	if err != nil {
		return nil, err
	}
	var s CfCredentials
	if err := json.Unmarshal(buf, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func getEndpoint(ctx context.Context, client client.Client, pc *v1beta1.ProviderConfig) (*string, error) {

	if pc.Spec.APIEndpoint != nil {
		return pc.Spec.APIEndpoint, nil
	}

	if pc.Spec.Endpoint != nil {
		buf, err := resource.CommonCredentialExtractor(ctx, pc.Spec.Endpoint.Source, client, pc.Spec.Endpoint.CommonCredentialSelectors)
		if err != nil {
			return nil, err
		}
		endpoint := string(buf)
		return &endpoint, nil
	}
	return nil, errors.New(errNoEndpoint)
}

type ClientFn func(resource.Managed) (*cfv3.Client, error)

func ClientFnBuilder(ctx context.Context, client client.Client) func(resource.Managed) (*cfv3.Client, error) {
	return func(mg resource.Managed) (*cfv3.Client, error) {
		cfg, err := GetCredentialConfig(ctx, client, mg)
		if err != nil {
			return nil, errors.Wrap(err, "cannot config cloudfoundry client")
		}

		return cfv3.New(cfg)
	}
}
