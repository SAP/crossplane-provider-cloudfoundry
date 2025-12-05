package main


import (
	"context"
	"log/slog"

	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/configparam"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/export"
)

func exportLogic(ctx context.Context, events export.EventHandler) error {
	slog.Info("export command invoked",
		"protocols", protocolParam.Value(),
		"username", usernameParam.Value(),
		"boolparam", boolParam.Value(),
	)

	events.Stop()
	return nil
}

var protocolParam = configparam.StringSlice("protocol", "list of supported protocols").
	WithShortName("p").
	WithEnvVarName("PROTOCOLS")

var usernameParam = configparam.String("username", "username used for authentication").
	WithShortName("u").
	WithEnvVarName("USERNAME")

var boolParam = configparam.Bool("bool", "test bool parameter").
	WithShortName("b").
	WithEnvVarName("CLIFFORD_BOOL")

func main() {
	cli.Configuration.ShortName = "test"
	cli.Configuration.ObservedSystem = "test system"
	export.AddConfigParams(protocolParam, usernameParam, boolParam)
	export.SetCommand(exportLogic)
	cli.Execute()
}
