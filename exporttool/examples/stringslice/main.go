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
		"protocols", testParam.Value(),
		"num-of-protos", len(testParam.Value()),
		"is-set", testParam.IsSet(),
	)

	events.Stop()
	return nil
}

var testParam = configparam.StringSlice("protocol", "list of supported protocols").
	WithShortName("p").
	WithEnvVarName("PROTOCOLS")

func main() {
	cli.Configuration.ShortName = "test"
	cli.Configuration.ObservedSystem = "test system"
	export.AddConfigParams(testParam)
	export.SetCommand(exportLogic)
        cli.Execute()
}
