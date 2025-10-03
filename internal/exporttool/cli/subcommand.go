package cli

import (
	"os"

	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/subcommand"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/erratt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func printConfiguration(cmd *cobra.Command) {
}

func makeCobraRun(fn func() error) func(*cobra.Command, []string) {
	return func(_ *cobra.Command, _ []string) {
		if err := fn(); err != nil {
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
						if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
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
