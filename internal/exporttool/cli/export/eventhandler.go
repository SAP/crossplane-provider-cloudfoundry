package export

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/erratt"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/yaml"

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
			erratt.SlogWarnWith(err, errlog)
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

type handler[T any] struct {
	ctx    context.Context
	closed bool
	ch     chan T
}

func newHandler[T any](ctx context.Context) *handler[T] {
	return &handler[T]{
		ctx:    ctx,
		closed: false,
		ch:     make(chan T),
	}
}

func (h *handler[T]) Event(event T) {
	if !h.closed {
		select {
		case h.ch <- event:
		case <-h.ctx.Done():
			h.Stop()
		}
	}
}

func (h *handler[T]) Stop() {
	if !h.closed {
		h.closed = true
		close(h.ch)
	}
}

type EventHandler interface {
	Error(erratt.Error)
	Resource(resource.Object)
	Stop()
}

type eventHandler struct {
	errorHandler    *handler[erratt.Error]
	resourceHandler *handler[resource.Object]
}

var _ EventHandler = eventHandler{}

func newEventHandler(ctx context.Context) eventHandler {
	return eventHandler{
		errorHandler:    newHandler[erratt.Error](ctx),
		resourceHandler: newHandler[resource.Object](ctx),
	}
}

func (eh eventHandler) Error(err erratt.Error) {
	eh.errorHandler.Event(err)
}

func (eh eventHandler) Resource(res resource.Object) {
	eh.resourceHandler.Event(res)
}

func (eh eventHandler) Stop() {
	eh.errorHandler.Stop()
	eh.resourceHandler.Stop()
}
