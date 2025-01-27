package fake

import (
	"context"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/stretchr/testify/mock"
)

// MockJob mocks Job service
type MockJob struct {
	mock.Mock
}

// PollComplete mocks Job.PollComplete
func (m *MockJob) PollComplete(ctx context.Context, job string, opt *client.PollingOptions) error {
	args := m.Called()
	return args.Error(0)
}
