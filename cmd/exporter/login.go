package main

import (
	"fmt"

	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/configparam"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/subcommand"
)

func login() error {
	fmt.Println("TODO: login command invoked")
	fmt.Println(loginSubCommand.ConfigParams.String())
	apiUrl, err := loginSubCommand.ConfigParams[0].(*configparam.StringParam).ValueOrAsk()
	if err != nil {
		return err
	}
	fmt.Printf("API URL = '%s'\n", apiUrl)
	return nil
}

var loginSubCommand = &subcommand.Simple{
	Name:  "login",
	Short: fmt.Sprintf("Logging in to %s cluster", observedSystem),
	Long:  fmt.Sprintf("Logging in to %s cluster", observedSystem),
	ConfigParams: configparam.ParamList{
		configparam.String("login API URL", "URL of the Cloud Foundry API").
			WithShortName("a").
			WithFlagName("apiUrl").
			WithEnvVarName("API_URL").
			WithExample("https://api.cf.enterprise.com"),
		configparam.Bool("testLong", "log test flag"),
		configparam.Bool("testShort", "log test short flag").WithShortName("s"),
		configparam.Bool("testOtherDefault", "log test other default").WithDefaultValue(true),
	},
}

func init() {
	loginSubCommand.Logic = login
	cli.RegisterSubCommand(loginSubCommand)
}

// type loginSubCommand struct{}

// var _ subcommand.SubCommand = loginSubCommand{}

// func (command loginSubCommand) GetName() string {
// 	return "login"
// }

// func (command loginSubCommand) GetShort() string {
// 	return fmt.Sprintf("Logging in to %s cluster", cli.Configuration.CLIConfiguration.ObservedSystem)
// }

// func (command loginSubCommand) GetLong() string {
// 	return fmt.Sprintf("Logging in to %s cluster", cli.Configuration.CLIConfiguration.ObservedSystem)
// }

// func (command loginSubCommand) GetConfigParams() []configparam.ConfigParam {
// 	return []configparam.ConfigParam{
// 		configparam.Bool("testLong", "log test flag"),
// 		configparam.Bool("testShort", "log test short flag").WithShortName("s"),
// 		configparam.Bool("testOtherDefault", "log test other default").WithDefaultValue(true),
// 	}
// }
