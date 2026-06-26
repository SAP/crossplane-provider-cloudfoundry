package fake

import (
	"context"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/stretchr/testify/mock"
)

type MockServicePlan struct {
	mock.Mock
}

func (m *MockServicePlan) Get(ctx context.Context, guid string) (*resource.ServicePlan, error) {
	args := m.Called(guid)
	return args.Get(0).(*resource.ServicePlan), args.Error(1)
}

func (m *MockServicePlan) Single(ctx context.Context, opts *client.ServicePlanListOptions) (*resource.ServicePlan, error) {
	args := m.Called()
	return args.Get(0).(*resource.ServicePlan), args.Error(1)
}
