package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

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
	runCommand              func(resourceChan chan<- resource.Object, errChan chan<- *erratt.Error) error
	configParams            configparam.ParamList
	exportableResourceKinds []string
}

var ResourceKindParam = configparam.StringSlice("exported kinds", "Resource kinds to export").
	WithShortName("k").
	WithFlagName("kind")

var OutputParam = configparam.String("output", "redirect the YAML output to a file").
	WithShortName("o").
	WithFlagName("output")

var (
	_         subcommand.SubCommand = &exportSubCommand{}
	exportCmd                       = &exportSubCommand{
		runCommand: func(_ chan<- resource.Object, _ chan<- *erratt.Error) error {
			return erratt.New("export subcommand is not set")
		},
		configParams: configparam.ParamList{
			ResourceKindParam,
			OutputParam,
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

func (c *exportSubCommand) MustIgnoreConfigFile() bool {
	return false
}

func printErrors(ctx context.Context, errChan <-chan *erratt.Error) {
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

func openOutput() (*os.File, *erratt.Error) {
	var fileOutput *os.File
	if o := OutputParam.Value(); o != "" {
		var err error
		fileOutput, err = os.Create(filepath.Clean(o))
		if err != nil {
			return nil, erratt.Errorf("Cannot create output file: %w", err).With("output", o)
		}

		slog.Info("Writing output to file", "output", o)
	}
	return fileOutput, nil
}

func resourceLoop(ctx context.Context, fileOutput *os.File, resourceChan <-chan resource.Object, errChan chan<- *erratt.Error) {
	for {
		select {
		case res, ok := <-resourceChan:
			if !ok {
				// resource channel is closed
				return
			}
			if fileOutput != nil {
				// output to file
				y, err := yaml.Marshal(res)
				if err != nil {
					errChan <- erratt.Errorf("cannot YAML-marshal resource: %w", err)
				} else {
					if _, err := fmt.Fprint(fileOutput, y); err != nil {
						errChan <- erratt.Errorf("cannot write YAML to output: %w", err).With("output", fileOutput.Name())
					}
				}
			} else {
				// output to console
				y, err := yaml.MarshalPretty(res)
				if err != nil {
					errChan <- erratt.Errorf("cannot YAML-marshal resource: %w", err)
				} else {
					fmt.Print(y)
				}
			}
		case <-ctx.Done():
			// execution is cancelled
			return
		}
	}
}

func handleResources(ctx context.Context, resourceChan <-chan resource.Object, errChan chan<- *erratt.Error) {
	fileOutput, err := openOutput()
	if err != nil {
		errChan <- err
	}
	defer func() {
		err := fileOutput.Close()
		if err != nil {
			errChan <- erratt.Errorf("Cannot close output file: %w", err).With("output", fileOutput.Name())
		}
	}()
	resourceLoop(ctx, fileOutput, resourceChan, errChan)
}

func (c *exportSubCommand) Run() func() error {
	return func() error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		errChan := make(chan *erratt.Error)
		go printErrors(ctx, errChan)
		resourceChan := make(chan resource.Object)
		go handleResources(ctx, resourceChan, errChan)
		return c.runCommand(resourceChan, errChan)
	}
}

func SetExportCommand(cmd func(resourceChan chan<- resource.Object, errChan chan<- *erratt.Error) error) {
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
