package clients

import (
	"context"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/runtime/client"

	cfclient "github.com/cloudfoundry/go-cfclient/v3/client"
)

type DefaultTokenFactory struct {
	ctx      context.Context
	cfClient *cfclient.Client
}

func NewDefaultTokenFactory(ctx context.Context, cfClient *cfclient.Client) *DefaultTokenFactory {
	return &DefaultTokenFactory{
		ctx:      ctx,
		cfClient: cfClient,
	}
}

// NewToken retrieves oauth token
func (t *DefaultTokenFactory) NewToken() (runtime.ClientAuthInfoWriter, error) {
	rawToken, err := t.NewRawToken()
	return client.BearerToken(rawToken), err
}

func (t *DefaultTokenFactory) NewRawToken() (string, error) {
	source, err := t.cfClient.CreateOAuth2TokenSource(t.ctx)
	if err != nil {
		return "", err
	}

	token, err := source.Token()
	if err != nil {
		return "", err
	}

	return token.AccessToken, nil
}
