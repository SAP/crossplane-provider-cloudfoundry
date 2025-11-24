package cli_test

import (
	"context"
	"fmt"

	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/configparam"
)

func ExampleRegisterSubCommand() {
	cli.Configuration.ShortName = "ts"
	cli.Configuration.ObservedSystem = "test system"
	subcommand := &cli.BasicSubCommand{
		Name:             "login",
		Short:            "login to test system",
		Long:             "perform a login to test system",
		ConfigParams:     []configparam.ConfigParam{},
		IgnoreConfigFile: false,
		Run: func(_ context.Context) error {
			fmt.Println("login subcommand invoked")
			return nil
		},
	}
	cli.RegisterSubCommand(subcommand)
	cli.Execute()
}
