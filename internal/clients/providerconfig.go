/*
Copyright 2023 SAP SE
*/

package clients

import (
	"context"
	"encoding/json"

	"github.com/cloudfoundry-community/go-cfclient/v3/config"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/upjet/pkg/terraform"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/apis/v1beta1"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/clients/cfclient"
)

// CfCredentials used to authenticate with the provider
// FIXME: not consistent with other providers.
type CfCredentials struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
	Passcode string `json:"passcode"`
}

// FIXME: keys do not match with btp-account connection details
const (
	keyBaseURL     = "api_url"
	keyUser        = "user"
	keyPassword    = "password"
	keySsoPasscode = "sso_passcode"
)

const (
	// error messages
	errNoProviderConfig     = "no providerConfigRef provided"
	errGetProviderConfig    = "cannot get referenced ProviderConfig"
	errTrackUsage           = "cannot track ProviderConfig usage"
	errExtractCredentials   = "cannot extract credentials"
	errExtractEndpoint      = "cannot extract endpoint"
	errUnmarshalCredentials = "cannot unmarshal cloudfoundry credentials as JSON"
	errUnmarshalEndpoint    = "cannot unmarshal cloudfoundry endpoint as JSON"
)

// TerraformSetupBuilder builds Terraform a terraform.SetupFn function which
// returns Terraform provider setup configuration
func TerraformSetupBuilder(version, providerSource, providerVersion string) terraform.SetupFn {
	return func(ctx context.Context, client client.Client, mg resource.Managed) (terraform.Setup, error) {
		ps := terraform.Setup{
			Version: version,
			Requirement: terraform.ProviderRequirement{
				Source:  providerSource,
				Version: providerVersion,
			},
		}

		pc, err := getProviderConfig(ctx, client, mg)
		if err != nil {
			return ps, errors.Wrap(err, errGetProviderConfig)
		}

		t := resource.NewProviderConfigUsageTracker(client, &v1beta1.ProviderConfigUsage{})
		if err := t.Track(ctx, mg); err != nil {
			return ps, errors.Wrap(err, errTrackUsage)
		}

		cred, err := getCredentials(ctx, client, pc)
		if err != nil {
			return ps, errors.Wrap(err, errExtractCredentials)
		}
		url, err := getEndpoint(ctx, client, pc)
		if err != nil {
			return ps, errors.Wrap(err, errExtractEndpoint)
		}

		ps.Configuration = map[string]any{}
		ps.Configuration[keyBaseURL] = *url
		// use email
		ps.Configuration[keyUser] = cred.Email
		ps.Configuration[keyPassword] = cred.Password
		ps.Configuration[keySsoPasscode] = cred.Passcode

		return ps, nil
	}
}

// CloudfoundryClientBuilder is a function that builds a CF Client
var CloudfoundryClientBuilder = func(ctx context.Context, client client.Client, mg resource.Managed) (*cfclient.Client, error) {
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

	cfg, err := config.NewUserPassword(*url, cred.Email, cred.Password)
	if err != nil {
		return nil, errors.Wrap(err, "cannot config cloudfoundry client")
	}
	cfg.WithSkipTLSValidation(true)

	return cfclient.New(cfg)
}

func getProviderConfig(ctx context.Context, client client.Client, mg resource.Managed) (*v1beta1.ProviderConfig, error) {
	pc := &v1beta1.ProviderConfig{}
	if err := client.Get(ctx, types.NamespacedName{Name: mg.GetProviderConfigReference().Name}, pc); err != nil {
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
	buf, err := resource.CommonCredentialExtractor(ctx, pc.Spec.Endpoint.Source, client, pc.Spec.Endpoint.CommonCredentialSelectors)
	if err != nil {
		return nil, err
	}
	endpoint := string(buf)
	return &endpoint, nil
}
