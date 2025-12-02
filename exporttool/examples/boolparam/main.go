package main


import (
	"context"
	"log/slog"

	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/configparam"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/export"
)

func exportLogic(_ context.Context, events export.EventHandler) error {
	slog.Info("export command invoked", "test-value", testParam.Value())
	events.Stop()
	return nil
}

var testParam = configparam.Bool("test", "test bool parameter").
        WithShortName("t").
        WithEnvVarName("CLIFFORD_TEST")

func main() {
	cli.Configuration.ShortName = "test"
	cli.Configuration.ObservedSystem = "test system"
        export.AddConfigParams(testParam)
	export.SetCommand(exportLogic)
        cli.Execute()
}
