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
	        "secure", secureParam.Value(),
		"secure-is-set", secureParam.IsSet(),
		"protocols", protocolsParam.Value(),
		"num-of-protos", len(protocolsParam.Value()),
		"protocols-is-set", protocolsParam.IsSet(),
	)

	protocols, err := protocolsParam.ValueOrAsk(ctx)
	if err != nil {
		return err
	}

	slog.Info("data acquired", "protocols", protocols)

	events.Stop()
	return nil
}

func possibleProtocols() ([]string, error) {
	if secureParam.Value() {
		return []string{"HTTPS", "SFTP", "SSH"}, nil
	}
	return []string{"FTP", "HTTP"}, nil
}

var secureParam = configparam.Bool("secure", "secure protocol").
        WithShortName("s").
        WithEnvVarName("SECURE")

var protocolsParam = configparam.StringSlice("protocol", "list of supported protocols").
	WithShortName("p").
	WithEnvVarName("PROTOCOLS").
	WithPossibleValuesFn(possibleProtocols)

func main() {
	cli.Configuration.ShortName = "test"
	cli.Configuration.ObservedSystem = "test system"
	export.AddConfigParams(secureParam, protocolsParam)
	export.SetCommand(exportLogic)
        cli.Execute()
}
