package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/configparam"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/subcommand"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/yaml"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/erratt"
	"github.com/charmbracelet/log"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

func init() {
	RegisterSubCommand(exportCmd)
}

type exportSubCommand struct {
	runCommand              func(resourceChan chan<- resource.Object, errChan chan<- erratt.ErrorWithAttrs) error
	configParams            configparam.ParamList
	exportableResourceKinds []string
}

var ResourceKindParam = configparam.StringSlice("exported kinds", "Resource kinds to export").
	WithShortName("k").
	WithFlagName("kind").(*configparam.StringSliceParam)

var (
	_         subcommand.SubCommand = &exportSubCommand{}
	exportCmd                       = &exportSubCommand{
		runCommand: func(_ chan<- resource.Object, _ chan<- erratt.ErrorWithAttrs) error {
			return erratt.New("export subcommand is not set")
		},
		configParams: configparam.ParamList{
			ResourceKindParam,
		},
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

func printErrors(ctx context.Context, errChan <-chan erratt.ErrorWithAttrs) {
	errlog := slog.New(log.NewWithOptions(os.Stderr, log.Options{}))
	for {
		select {
		case err, ok := <-errChan:
			if !ok {
				// error channel is closed
				return
			}
			erratt.SlogWith(err, errlog)
		case <-ctx.Done():
			// execution is cancelled
			return
		}
	}
}

func handleResources(ctx context.Context, resourceChan <-chan resource.Object, errChan chan<- erratt.ErrorWithAttrs) {
	for {
		select {
		case res, ok := <-resourceChan:
			if !ok {
				// resource channel is closed
				return
			}
			y, err := yaml.Marshal(res)
			if err != nil {
				errChan <- erratt.Errorf("cannot YAML-marshal resource: %w", err)
			} else {
				fmt.Println(y)
			}
		case <-ctx.Done():
			// execution is cancelled
			return
		}
	}
}

func (c *exportSubCommand) Run() func() error {
	return func() error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		errChan := make(chan erratt.ErrorWithAttrs)
		go printErrors(ctx, errChan)
		resourceChan := make(chan resource.Object)
		go handleResources(ctx, resourceChan, errChan)
		return c.runCommand(resourceChan, errChan)
	}
}

func SetExportCommand(cmd func(resourceChan chan<- resource.Object, errChan chan<- erratt.ErrorWithAttrs) error) {
	exportCmd.runCommand = cmd
}

func AddExportCommandParams(param ...configparam.ConfigParam) {
	exportCmd.configParams = append(exportCmd.configParams, param...)
}

func GetExportConfigParams() configparam.ParamList {
	return exportCmd.configParams
}

func AddExportableResourceKinds(kinds ...string) {
	exportCmd.exportableResourceKinds = append(exportCmd.exportableResourceKinds, kinds...)
	ResourceKindParam.WithPossibleValues(exportCmd.exportableResourceKinds)
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
