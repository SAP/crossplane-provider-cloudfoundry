package config

import (
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/configparam"
	"github.com/cloudfoundry/go-cfclient/v3/config"
)

func Get(apiUrlParam, usernameParam, passwordParam *configparam.StringParam) (*config.Config, error) {
	apiUrl, err := apiUrlParam.ValueOrAsk()
	if err != nil {
		return nil, err
	}
	username, err := usernameParam.ValueOrAsk()
	if err != nil {
		return nil, err
	}
	password, err := passwordParam.ValueOrAsk()
	if err != nil {
		return nil, err
	}
	return config.New(apiUrl, config.UserPassword(username, password))
}
