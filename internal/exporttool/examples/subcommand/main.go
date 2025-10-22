package main

import (
	"context"
	"log/slog"

	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/configparam"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/widget"
)

var subcommand = &cli.BasicSubCommand{
	Name:             "widget",
	Short:            "widget testing",
	Long:             "demo widget capabilities",
	ConfigParams:     []configparam.ConfigParam{},
	Run: widgetTesting,
}

func widgetTesting(ctx context.Context) error {
	slog.Info("widget testing")
	_, err := widget.TextInput(ctx, "Testing TextInput", "enter text", false)
	if err != nil {
		return err
	}
	_, err = widget.TextInput(ctx, "Testing sensitive TextInput", "", true)
	if err != nil {
		return err
	}
	_, err = widget.MultiInput(ctx, "Testing MultiInput", []string{"option A", "option B", "option C"})
	if err != nil {
		return err
	}
	return nil
}

func main() {
	cli.Configuration.ShortName = "test"
	cli.Configuration.ObservedSystem = "test system"
	cli.RegisterSubCommand(subcommand)
	cli.Execute()
}
