package main

import (
	"context"
	"fmt"

	"github.com/SAP/xp-clifford/cli"
	"github.com/SAP/xp-clifford/cli/configparam"
	"github.com/SAP/xp-clifford/erratt"
)

func login(ctx context.Context) error {
	apiURL, err := apiURLParam.ValueOrAsk(ctx)
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
	cfg.Set(apiURLParam.FlagName, apiURL)
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
		apiURLParam,
		usernameParam,
		passwordParam,
	},
}

func init() {
	loginSubCommand.Run = login
	cli.RegisterSubCommand(loginSubCommand)
}
