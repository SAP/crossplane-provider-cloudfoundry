package export

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/yaml"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/erratt"

	"github.com/charmbracelet/log"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

func printErrors(ctx context.Context, wg *sync.WaitGroup, errChan <-chan erratt.Error) {
	defer wg.Done()
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

func openOutput() (*os.File, erratt.Error) {
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

func resourceLoop(ctx context.Context, fileOutput *os.File, resourceChan <-chan resource.Object, errChan chan<- erratt.Error) {
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

func handleResources(ctx context.Context, wg *sync.WaitGroup, resourceChan <-chan resource.Object, errChan chan<- erratt.Error) {
	defer wg.Done()
	fileOutput, err := openOutput()
	if err != nil {
		errChan <- err
	}
	defer func() {
		if fileOutput != nil {
			err := fileOutput.Close()
			if err != nil {
				errChan <- erratt.Errorf("Cannot close output file: %w", err).With("output", fileOutput.Name())
			}
		}
	}()
	resourceLoop(ctx, fileOutput, resourceChan, errChan)
}

func (c *exportSubCommand) Run() func() error {
	return func() error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		// errChan := make(chan erratt.Error)
		evHandler := newEventHandler()
		wg := sync.WaitGroup{}
		wg.Add(1)
		go printErrors(ctx, &wg, evHandler.errorHandler)
		// resourceChan := make(chan resource.Object)

		wg.Add(1)
		go handleResources(ctx, &wg, evHandler.resourceHandler, evHandler.errorHandler)
		err := c.runCommand(evHandler)
		if err != nil {
			return err
		}
		wg.Wait()
		return nil
	}
}

type errorHandler chan erratt.Error

func newErrorHandler() errorHandler {
	return make(chan erratt.Error)
}

func (eh errorHandler) Error(err erratt.Error) {
	eh <- err
}

type resourceHandler chan resource.Object

func newResourceHandler() resourceHandler {
	return make(chan resource.Object)
}

func (rh resourceHandler) Resource(res resource.Object) {
	rh <- res
}

type EventHandler interface {
	Error(erratt.Error)
	Resource(resource.Object)
	Stop()
}

type eventHandler struct {
	errorHandler
	resourceHandler
}

var _ EventHandler = eventHandler{}

func newEventHandler() eventHandler {
	return eventHandler{
		errorHandler:    newErrorHandler(),
		resourceHandler: newResourceHandler(),
	}
}

func (eh eventHandler) Stop() {
	close(eh.errorHandler)
	close(eh.resourceHandler)
}
