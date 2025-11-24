package main

import (
	"context"
	"fmt"

	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/configparam"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/erratt"
)

func login(ctx context.Context) error {
	apiUrl, err := apiUrlParam.ValueOrAsk(ctx)
	if err != nil {
		return erratt.New("Cannot get API URL parameter").With("subcommand", "login")
	}
	username, err := usernameParam.ValueOrAsk(ctx)
	if err != nil {
		return erratt.New("Cannot get username parameter")
	}
	password, err := passwordParam.ValueOrAsk(ctx)
	if err != nil {
		return erratt.New("Cannot get password parameter")
	}

	cfg := cli.ConfigFileSettings{}
	cfg.Set(apiUrlParam.FlagName, apiUrl)
	cfg.Set(usernameParam.FlagName, username)
	cfg.Set(passwordParam.FlagName, password)
	return cfg.StoreConfig(cli.ConfigFileParam.Value())
}

var loginSubCommand = &cli.BasicSubCommand{
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
	loginSubCommand.Run = login
	cli.RegisterSubCommand(loginSubCommand)
}
