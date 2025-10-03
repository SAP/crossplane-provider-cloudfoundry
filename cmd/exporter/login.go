package main

import (
	"fmt"

	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/configparam"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/subcommand"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/erratt"
)

func login() error {
	apiUrl, err := apiUrlParam.ValueOrAsk()
	if err != nil {
		return erratt.New("Cannot get API URL parameter").With("subcommand", "login")
	}
	username, err := usernameParam.ValueOrAsk()
	if err != nil {
		return erratt.New("Cannot get username parameter")
	}
	password, err := passwordParam.ValueOrAsk()
	if err != nil {
		return erratt.New("Cannot get password parameter")
	}

	cfg := cli.ConfigFileSettings{}
	cfg.Set(apiUrlParam.FlagName, apiUrl)
	cfg.Set(usernameParam.FlagName, username)
	cfg.Set(passwordParam.FlagName, password)
	return cfg.StoreConfig(cli.ConfigFileParam.Value())
}

var loginSubCommand = &subcommand.Simple{
	Name:             "login",
	Short:            fmt.Sprintf("Logging in to %s cluster", observedSystem),
	Long:             fmt.Sprintf("Logging in to %s cluster", observedSystem),
	IgnoreConfigFile: true,
	ConfigParams: configparam.ParamList{
		apiUrlParam,
		usernameParam,
		passwordParam,
	},
}

func init() {
	loginSubCommand.Logic = login
	cli.RegisterSubCommand(loginSubCommand)
}
