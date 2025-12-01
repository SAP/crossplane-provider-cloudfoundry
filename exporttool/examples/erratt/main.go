package main


import (
	"context"
	"log/slog"

	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/export"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/erratt"
)

func auth() erratt.Error {
        return erratt.New("authenication failure",
		"username", "test-user",
		"password", "test-password",
	)
}

func connect() erratt.Error {
	err := auth()
	if err != nil {
		return erratt.Errorf("connect failed: %w", err).
			With("url", "https://example.com")
	}
	return nil
}

func exportLogic(_ context.Context, events export.EventHandler) error {
	slog.Info("export command invoked")

	err := connect()

	events.Stop()
	return err
}

func main() {
	cli.Configuration.ShortName = "test"
	cli.Configuration.ObservedSystem = "test system"
	export.SetCommand(exportLogic)
	cli.Execute()
}
