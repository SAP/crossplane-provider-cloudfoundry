package cli

import (
	"fmt"

	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/configparam"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/subcommand"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/erratt"
)

func init() {
	RegisterSubCommand(exportCmd)
}

type exportSubCommand struct {
	runCommand   func() error
	configParams configparam.ParamList
}

var (
	_         subcommand.SubCommand = &exportSubCommand{}
	exportCmd                       = &exportSubCommand{
		runCommand: func() error {
			return erratt.New("export subcommand is not set")
		},
		configParams: configparam.ParamList{},
	}
)

func (c *exportSubCommand) GetName() string {
	return "export"
}

func (c *exportSubCommand) GetShort() string {
	return fmt.Sprintf("Export %s resources", Configuration.CLIConfiguration.ObservedSystem)
}

func (c *exportSubCommand) GetLong() string {
	return fmt.Sprintf("Export %s resources and transform them into managed resources that the Crossplane provider can consume", Configuration.CLIConfiguration.ObservedSystem)
}

func (c *exportSubCommand) GetConfigParams() configparam.ParamList {
	return c.configParams
}

func (c *exportSubCommand) Run() func() error {
	return c.runCommand
}

func SetExportCommand(cmd func() error) {
	exportCmd.runCommand = cmd
}

func AddExportCommandParams(param... configparam.ConfigParam) {
	exportCmd.configParams = append(exportCmd.configParams, param...)
}

func GetExportConfigParams() configparam.ParamList {
	return exportCmd.configParams
}

// var exportCmd *cobra.Command

// type defaultConfiguratorExportSubcommand struct{}

// var _ ConfiguratorExportSubcommand = defaultConfiguratorExportSubcommand{}

// func (c defaultConfiguratorExportSubcommand) CommandShort(config *ConfigSchema) string {
// 	return fmt.Sprintf("Export %s resources", config.CLIConfiguration.ObservedSystem)
// }

// func (c defaultConfiguratorExportSubcommand) CommandLong(config *ConfigSchema) string {
// 	return fmt.Sprintf("Export %s resources and transform them into managed resources that the Crossplane provider can consume", config.CLIConfiguration.ObservedSystem)
// }

// type ExportSubcommandConfiguration struct {
// 	ConfiguratorExportSubcommand

// 	// SubcommandName
// 	SubcommandName string
// }

// func DefaultExportSubcommandConfiguration() ExportSubcommandConfiguration {
// 	return ExportSubcommandConfiguration{
// 		ConfiguratorExportSubcommand: defaultConfiguratorExportSubcommand{},
// 		SubcommandName: "export",
// 	}
// }

// func configureExportSubcommand() error {
// 	config := Configuration.ExportSubcommandConfiguration
// 	exportCmd = &cobra.Command{
// 		Use:   config.SubcommandName,
// 		Short: config.CommandShort(Configuration),
// 		Long:  config.CommandLong(Configuration),
// 	}
// 	Command.AddCommand(exportCmd)
// 	return nil
// }
