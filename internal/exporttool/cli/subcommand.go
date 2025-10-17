package cli

import (
	"context"
	"errors"
	"os"

	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/subcommand"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/erratt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func makeCobraRun(fn func(context.Context) error) func(*cobra.Command, []string) {
	return func(_ *cobra.Command, _ []string) {
		if err := fn(CliCtx); err != nil {
			erratt.Slog(err)
			os.Exit(1)
		}
	}
}

func RegisterSubCommand(command subcommand.SubCommand) {
	RegisterConfigModule(func() error {
		cmd := &cobra.Command{
			Use:   command.GetName(),
			Short: command.GetShort(),
			Long:  command.GetLong(),
			PreRun: func(cmd *cobra.Command, _ []string) {
				for _, cp := range command.GetConfigParams() {
					cp.BindConfiguration(cmd)
				}
				if !command.MustIgnoreConfigFile() {
					if err := viper.ReadInConfig(); err != nil {
						if !errors.Is(err, viper.ConfigFileNotFoundError{}) {
							erratt.Slog(erratt.New("cannot read config file"))
						}
					}
				}
			},
			Run: makeCobraRun(command.Run()),
		}
		Command.AddCommand(cmd)
		for _, cp := range command.GetConfigParams() {
			cp.AttachToCommand(cmd)
		}
		return nil
	})
}
