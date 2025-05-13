package fake

import (
	"context"
	"encoding/json"
	"time"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/stretchr/testify/mock"
)

// MockServiceInstance mocks ServiceInstance interfaces
type MockServiceInstance struct {
	mock.Mock
}

// Get mocks ServiceInstance.Get
func (m *MockServiceInstance) Get(ctx context.Context, guid string) (*resource.ServiceInstance, error) {
	args := m.Called(guid)
	return args.Get(0).(*resource.ServiceInstance), args.Error(1)
}

// GetManagedParameters mocks ServiceInstance.GetManagedParameters
func (m *MockServiceInstance) GetManagedParameters(ctx context.Context, guid string) (*json.RawMessage, error) {
	args := m.Called(guid)
	return args.Get(0).(*json.RawMessage), args.Error(1)
}

// GetUserProvidedCredentials mocks ServiceInstance.GetUserProvidedCredentials
func (m *MockServiceInstance) GetUserProvidedCredentials(ctx context.Context, guid string) (*json.RawMessage, error) {
	args := m.Called(guid)
	return args.Get(0).(*json.RawMessage), args.Error(1)
}

// Single mocks ServiceInstance.Single
func (m *MockServiceInstance) Single(ctx context.Context, opt *client.ServiceInstanceListOptions) (*resource.ServiceInstance, error) {
	args := m.Called()
	return args.Get(0).(*resource.ServiceInstance), args.Error(1)
}

// CreateManaged mocks ServiceInstance.CreateManaged
func (m *MockServiceInstance) CreateManaged(ctx context.Context, opt *resource.ServiceInstanceManagedCreate) (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

// CreateUserProvided mocks ServiceInstance.CreateUserProvided
func (m *MockServiceInstance) CreateUserProvided(ctx context.Context, opt *resource.ServiceInstanceUserProvidedCreate) (*resource.ServiceInstance, error) {
	args := m.Called(opt)
	return args.Get(0).(*resource.ServiceInstance), args.Error(1)
}

// UpdateManaged mocks ServiceInstance.UpdateManaged
func (m *MockServiceInstance) UpdateManaged(ctx context.Context, guid string, opt *resource.ServiceInstanceManagedUpdate) (string, *resource.ServiceInstance, error) {
	args := m.Called(guid)
	return args.String(0), nil, args.Error(1)
}

// UpdateUserProvided mocks ServiceInstance.UpdateUserProvided
func (m *MockServiceInstance) UpdateUserProvided(ctx context.Context, guid string, opt *resource.ServiceInstanceUserProvidedUpdate) (*resource.ServiceInstance, error) {
	args := m.Called(guid)
	return args.Get(0).(*resource.ServiceInstance), args.Error(1)
}

// Delete mocks ServiceInstance.Delete
func (m *MockServiceInstance) Delete(ctx context.Context, guid string) (string, error) {
	args := m.Called(guid)
	return args.String(0), args.Error(1)
}

// PollComplete mocks ServiceInstance.PollComplete
func (m *MockServiceInstance) PollComplete(ctx context.Context, job string, opt *client.PollingOptions) error {
	args := m.Called()
	return args.Error(0)
}

// JSONRawMessage returns a pointer to a json.RawMessage
func JSONRawMessage(s string) *json.RawMessage {
	if s == "" {
		return nil
	}
	j := json.RawMessage(s)
	return &j
}

// ServiceInstance is a nil ServiceInstance
var (
	ServiceInstanceNil *resource.ServiceInstance
)

// ServiceInstance is a ServiceInstance object
type ServiceInstance struct {
	resource.ServiceInstance
}

// NewServiceInstance generate a new ServiceInstance
func NewServiceInstance(t string) *ServiceInstance {
	r := &ServiceInstance{}
	r.Type = t
	return r
}

// SetName assigns ServiceInstance name
func (s *ServiceInstance) SetName(name string) *ServiceInstance {
	s.Name = name
	return s
}

// SetGUID assigns ServiceInstance GUID
func (s *ServiceInstance) SetGUID(guid string) *ServiceInstance {
	s.GUID = guid
	return s
}

// SetServicePlan assigns ServiceInstance ServicePlan
func (s *ServiceInstance) SetServicePlan(guid string) *ServiceInstance {
	s.Relationships.ServicePlan = &resource.ToOneRelationship{
		Data: &resource.Relationship{GUID: guid}}
	return s
}

// SetLastOperation assigns ServiceInstance LastOperation
func (s *ServiceInstance) SetLastOperation(op, state string) *ServiceInstance {
	s.LastOperation = resource.LastOperation{
		Type:        op,
		State:       state,
		Description: op + " " + state,
		UpdatedAt:   time.Now(),
	}
	return s
}
