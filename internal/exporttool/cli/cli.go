package cli

import (
	"fmt"
	"os"

	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/configparam"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/erratt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	RegisterConfigModule(configureCLI)
	Configuration.CLIConfiguration = defaultCLIConfiguration()
}

var Command *cobra.Command

func Execute() {
	if err := configure(); err != nil {
		erratt.Slog(err)
		os.Exit(1)
	}
	configureLogging()
	if err := Command.Execute(); err != nil {
		erratt.Slog(err)
		os.Exit(1)
	}
}

type CLIConfiguration struct {
	ConfiguratorCLI

	// ShortName is the abbreviated name of the observed system
	// that does not contain spaces, like "cf" for CloudFoundry
	// provider
	ShortName string

	// ObservedSystem is the full name of the external system that
	// may contain spaces, like "Cloud Foundry"
	ObservedSystem string

	HasVerboseFlag bool
}

func defaultCLIConfiguration() CLIConfiguration {
	return CLIConfiguration{
		ConfiguratorCLI: DefaultConfiguratorCLI{},
		ShortName:       "SHORTNAME_NOT_SET",
		ObservedSystem:  "OBSERVED_SYSTEM_NOT_SET",
		HasVerboseFlag:  true,
	}
}

type ConfiguratorCLI interface {
	CommandUse(config *ConfigSchema) string
	CommandShort(config *ConfigSchema) string
	CommandLong(config *ConfigSchema) string
}

type DefaultConfiguratorCLI struct{}

var _ ConfiguratorCLI = DefaultConfiguratorCLI{}

func (c DefaultConfiguratorCLI) CommandUse(config *ConfigSchema) string {
	return fmt.Sprintf("%s-exporter [command] [flags...]", config.ShortName)
}

func (c DefaultConfiguratorCLI) CommandShort(config *ConfigSchema) string {
	return fmt.Sprintf("%s exporting tool", config.ObservedSystem)
}

func (c DefaultConfiguratorCLI) CommandLong(config *ConfigSchema) string {
	return fmt.Sprintf("%s exporting tool is a CLI tool for exporting existing resources as Crossplane managed resources",
		config.ObservedSystem)
}

func configureCLI() error {
	config := Configuration.CLIConfiguration
	Command = &cobra.Command{
		Use:   config.CommandUse(Configuration),
		Short: config.CommandShort(Configuration),
		Long:  config.CommandLong(Configuration),
	}
	if config.HasVerboseFlag {
		// Command.Flags().BoolP("verbose", "v", false, "Verbose output")
		configparam.Bool("verbose", "Verbose output").WithShortName("v").AttachToCommand(Command)
	}

	return viper.BindPFlags(Command.PersistentFlags())
}
