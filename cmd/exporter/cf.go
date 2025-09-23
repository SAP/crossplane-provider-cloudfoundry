package main

import (
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/configparam"
	"github.com/cloudfoundry/go-cfclient/v3/config"
)

func getCFConfig() (*config.Config, error) {
	apiUrl, err := apiUrlParam.(*configparam.StringParam).ValueOrAsk()
	if err != nil {
		return nil, err
	}
	username, err := usernameParam.(*configparam.StringParam).ValueOrAsk()
	if err != nil {
		return nil, err
	}
	password, err := passwordParam.(*configparam.StringParam).ValueOrAsk()
	if err != nil {
		return nil, err
	}
	return config.New(apiUrl, config.UserPassword(username, password))
}
