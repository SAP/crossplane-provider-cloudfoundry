package config

import (
	"context"

	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/configparam"

	"github.com/cloudfoundry/go-cfclient/v3/config"
)

func Get(ctx context.Context, apiUrlParam, usernameParam, passwordParam *configparam.StringParam) (*config.Config, error) {
	apiUrl, err := apiUrlParam.ValueOrAsk(ctx)
	if err != nil {
		return nil, err
	}
	username, err := usernameParam.ValueOrAsk(ctx)
	if err != nil {
		return nil, err
	}
	password, err := passwordParam.ValueOrAsk(ctx)
	if err != nil {
		return nil, err
	}
	return config.New(apiUrl, config.UserPassword(username, password))
}
