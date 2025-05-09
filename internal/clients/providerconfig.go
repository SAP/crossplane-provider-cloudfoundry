/*
Copyright 2023 SAP SE
*/

package clients

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	mtaCsrf "github.com/cloudfoundry-incubator/multiapps-cli-plugin/clients/csrf"
	mtaClient "github.com/cloudfoundry-incubator/multiapps-cli-plugin/clients/mtaclient"

	cfv3 "github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/config"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
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

	errParseUrl = "cannot parse CF API URL"
	errNoUrl    = "could not parse mtarUrl"
	notFound    = "404 Not Found"

	deploy_service_host = "deploy-service"
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

	return config.New(*url, config.UserPassword(cred.Email, cred.Password), config.SkipTLSValidation())
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

func ClientFnBuilderMta(ctx context.Context, client client.Client, spaceId *string) func(resource.Managed) (*mtaClient.MtaClientOperations, error) {

	return func(mg resource.Managed) (*mtaClient.MtaClientOperations, error) {
		cfClient, err := ClientFnBuilder(ctx, client)(mg)
		if err != nil {
			return nil, err
		}

		urlObj, err := url.Parse(cfClient.ApiURL(""))
		if err != nil {
			return nil, errors.Wrap(err, errParseUrl)
		}

		tokenFactory := NewDefaultTokenFactory(ctx, cfClient)

		domainSeparatorIndex := strings.IndexByte(urlObj.Host, '.')
		domain := urlObj.Host[domainSeparatorIndex+1:]

		csrfx := mtaCsrf.CsrfTokenHelper{NonProtectedMethods: map[string]struct{}{"GET": {}, "HEAD": {}, "TRACE": {}, "OPTIONS": {}}}
		httpTransport := http.DefaultTransport.(*http.Transport).Clone()
		// Increase tls handshake timeout to cope with slow internet connections. 3 x default value = 30s.
		httpTransport.TLSHandshakeTimeout = 30 * time.Second
		httpTransport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: false,
			MinVersion:         tls.VersionTLS13,
		}
		transport := &mtaCsrf.Transport{Delegate: httpTransport, Csrf: &csrfx}

		mtaClientOperations := mtaClient.NewMtaClient(deploy_service_host+"."+domain, *spaceId, transport, tokenFactory)

		return &mtaClientOperations, nil
	}
}
