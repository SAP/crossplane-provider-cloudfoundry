package fake

import (
	"context"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/stretchr/testify/mock"
)

// MockApp mocks App interfaces
type MockApp struct {
	mock.Mock
}

// Get mocks App.Get
func (m *MockApp) Get(ctx context.Context, guid string) (*resource.App, error) {
	args := m.Called(guid)
	return args.Get(0).(*resource.App), args.Error(1)
}

// Single mocks App.Single
func (m *MockApp) Single(ctx context.Context, opt *client.AppListOptions) (*resource.App, error) {
	args := m.Called()
	return args.Get(0).(*resource.App), args.Error(1)
}

// Create mocks App.Create
func (m *MockApp) Create(ctx context.Context, opt *resource.AppCreate) (*resource.App, error) {
	args := m.Called()
	return args.Get(0).(*resource.App), args.Error(1)
}

// Update mocks App.Update
func (m *MockApp) Update(ctx context.Context, guid string, opt *resource.AppUpdate) (*resource.App, error) {
	args := m.Called(guid)
	return args.Get(0).(*resource.App), args.Error(1)
}

// Stop mocks App.Stop
func (m *MockApp) Stop(ctx context.Context, guid string) (*resource.App, error) {
	args := m.Called(guid)
	return args.Get(0).(*resource.App), args.Error(1)
}

// Start mocks App.Start
func (m *MockApp) Start(ctx context.Context, guid string) (*resource.App, error) {
	args := m.Called(guid)
	return args.Get(0).(*resource.App), args.Error(1)
}

// Delete mocks App.Delete
func (m *MockApp) Delete(ctx context.Context, guid string) (string, error) {
	args := m.Called(guid)
	return args.String(0), args.Error(1)
}

// PollComplete mocks App.PollComplete
func (m *MockApp) PollComplete(ctx context.Context, job string, opt *client.PollingOptions) error {
	args := m.Called()
	return args.Error(0)
}

// App is a nil App
var (
	AppNil *resource.App
)

// App is a App object
type App struct {
	resource.App
}

// NewApp generate a new App
func NewApp(t string) *App {
	r := &App{}
	r.Lifecycle.Type = t
	return r
}

// SetName assigns App name
func (a *App) SetName(name string) *App {
	a.Name = name
	return a
}

// SetGUID assigns App GUID
func (a *App) SetGUID(guid string) *App {
	a.GUID = guid
	return a
}
