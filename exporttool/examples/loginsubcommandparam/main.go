package main


import (
	"context"
	"log/slog"

	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/configparam"
	_ "github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/export"
)

func login(_ context.Context) error {
	slog.Info("login invoked", "test", testParam.Value())
	return nil
}

var testParam = configparam.Bool("test", "test bool parameter").
        WithShortName("t").
        WithEnvVarName("CLIFFORD_TEST")

func main() {
	cli.Configuration.ShortName = "test"
	cli.Configuration.ObservedSystem = "test system"

	var loginSubCommand = &cli.BasicSubCommand{
		Name:         "login",
		Short:        "Login demo subcommand",
		Long:         "A subcommand demonstrating xp-clifford capabilities",
		ConfigParams: []configparam.ConfigParam{
			testParam,
		},
		Run:          login,
	}

	cli.RegisterSubCommand(loginSubCommand)

	cli.Execute()
}
