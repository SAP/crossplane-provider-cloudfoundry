package fake

import (
	"context"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/stretchr/testify/mock"
)

// MockRoute mocks Route interfaces
type MockRoute struct {
	mock.Mock
}

// Get mocks Route.Get
func (m *MockRoute) Get(ctx context.Context, guid string) (*resource.Route, error) {
	args := m.Called(guid)
	return args.Get(0).(*resource.Route), args.Error(1)
}

// Single mocks Route.Single
func (m *MockRoute) Single(ctx context.Context, opt *client.RouteListOptions) (*resource.Route, error) {
	args := m.Called()
	return args.Get(0).(*resource.Route), args.Error(1)
}

// Create mocks Route.Create
func (m *MockRoute) Create(ctx context.Context, opt *resource.RouteCreate) (*resource.Route, error) {
	args := m.Called()
	return args.Get(0).(*resource.Route), args.Error(1)
}

// Update mocks Route.Update
func (m *MockRoute) Update(ctx context.Context, guid string, opt *resource.RouteUpdate) (*resource.Route, error) {
	args := m.Called()
	return args.Get(0).(*resource.Route), args.Error(1)
}

// Delete mocks Route.Delete
func (m *MockRoute) Delete(ctx context.Context, guid string) (string, error) {
	args := m.Called()
	return args.Get(0).(string), args.Error(1)
}

// Route is a nil Route
var (
	RouteNil *resource.Route
)

// FakeRoute generate a new Route
func FakeRoute(guid, url string) *resource.Route {
	r := &resource.Route{
		Resource: resource.Resource{
			GUID: guid,
		},
		URL: url,
	}
	return r
}
