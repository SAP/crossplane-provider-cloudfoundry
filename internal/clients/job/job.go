package job

import (
	"context"
	"errors"
	"time"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/config"
)

// Job defines interfaces to async operations/jobs.
type Job interface {
	PollComplete(ctx context.Context, jobGUID string, opt *client.PollingOptions) error
}

// NewClient returns a new CF Job client
func NewClient(config *config.Config) (Job, error) {
	cf, err := client.New(config)
	if err != nil {
		return nil, err
	}

	return cf.Jobs, nil
}

// newPollingOptions creates a new polling options with a timeout
var pollInterval = time.Second * 10
var pollTimeout = time.Minute * 1 // this can be shorter than creation time because we have logic to observe async operation state

func newPollingOptions() *client.PollingOptions {
	p := client.NewPollingOptions()
	p.Timeout = pollTimeout
	p.CheckInterval = pollInterval
	return p
}

// PollJobComplete polls for completion with extended timeout
func PollJobComplete(ctx context.Context, job Job, jobGUID string) error {
	ctx, cancel := context.WithTimeout(ctx, pollTimeout)
	defer cancel()

	err := job.PollComplete(ctx, jobGUID, newPollingOptions())

	if err != nil && errors.Is(err, client.AsyncProcessTimeoutError) { // because we have logic to observe job state, we can safely ignore timeout error
		return nil
	}

	return err
}
