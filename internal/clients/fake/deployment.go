package fake

import (
	"context"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/stretchr/testify/mock"
)

// MockDeployment mocks the app DeploymentClient interface.
type MockDeployment struct {
	mock.Mock
}

// Create mocks DeploymentClient.Create
func (m *MockDeployment) Create(ctx context.Context, r *resource.DeploymentCreate) (*resource.Deployment, error) {
	args := m.Called(r)
	return args.Get(0).(*resource.Deployment), args.Error(1)
}

// Get mocks DeploymentClient.Get
func (m *MockDeployment) Get(ctx context.Context, guid string) (*resource.Deployment, error) {
	args := m.Called(guid)
	return args.Get(0).(*resource.Deployment), args.Error(1)
}

// List mocks DeploymentClient.List
func (m *MockDeployment) List(ctx context.Context, opts *client.DeploymentListOptions) ([]*resource.Deployment, *client.Pager, error) {
	args := m.Called(opts)
	return args.Get(0).([]*resource.Deployment), nil, args.Error(1)
}
