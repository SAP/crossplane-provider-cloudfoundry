package fake

import (
	"context"
	"io"

	"github.com/cloudfoundry/go-cfclient/v3/operation"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/stretchr/testify/mock"
)

// MockPush mocks App interfaces
type MockPush struct {
	mock.Mock
}

// Get mocks PushClient.Push
func (m *MockPush) Get(ctx context.Context, guid string) (*resource.App, error) {
	args := m.Called(guid)
	return args.Get(0).(*resource.App), args.Error(1)
}

// Push mocks PushClient.Push
func (m *MockPush) Push(ctx context.Context, application *resource.App, manifest *operation.AppManifest, zipfile io.Reader) (*resource.App, error) {
	args := m.Called()
	return args.Get(0).(*resource.App), args.Error(1)
}

func (m *MockPush) GenerateManifest(ctx context.Context, appGUID string) (string, error) {
	args := m.Called(appGUID)
	return args.String(0), args.Error(1)
}
