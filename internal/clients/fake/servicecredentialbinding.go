package fake

import (
	"context"
	"time"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/stretchr/testify/mock"
)

// var _ ServiceCredentialBinding = &MockServiceCredentialBinding{}
var testTime = time.Now()

// MockServiceCredentialBinding mocks ServiceCredentialBinding interfaces
type MockServiceCredentialBinding struct {
	mock.Mock
}

// Get mocks ServiceCredentialBinding.Get
func (m *MockServiceCredentialBinding) Get(ctx context.Context, guid string) (*resource.ServiceCredentialBinding, error) {
	args := m.Called(ctx, guid)
	return args.Get(0).(*resource.ServiceCredentialBinding), args.Error(1)
}

// PollComplete mocks Job.PollComplete
func (m *MockServiceCredentialBinding) PollComplete(ctx context.Context, jobGUID string, opt *client.PollingOptions) error {
	args := m.Called()
	return args.Error(0)
}

// GetDetails mocks ServiceCredentialBinding.Get
func (m *MockServiceCredentialBinding) GetDetails(ctx context.Context, guid string) (*resource.ServiceCredentialBindingDetails, error) {
	args := m.Called(guid)
	return args.Get(0).(*resource.ServiceCredentialBindingDetails), args.Error(1)
}

// GetParameters mocks ServiceCredentialBinding.GetParameters
func (m *MockServiceCredentialBinding) GetParameters(ctx context.Context, guid string) (map[string]string, error) {
	args := m.Called(guid)
	return args.Get(0).(map[string]string), args.Error(1)
}

// Update mocks ServiceCredentialBinding.Update
func (m *MockServiceCredentialBinding) Update(ctx context.Context, guid string, r *resource.ServiceCredentialBindingUpdate) (*resource.ServiceCredentialBinding, error) {
	args := m.Called()
	return args.Get(0).(*resource.ServiceCredentialBinding), args.Error(1)
}

// Single mocks ServiceCredentialBinding.Single
func (m *MockServiceCredentialBinding) Single(ctx context.Context, opt *client.ServiceCredentialBindingListOptions) (*resource.ServiceCredentialBinding, error) {
	args := m.Called()
	return args.Get(0).(*resource.ServiceCredentialBinding), args.Error(1)
}

// Create mocks ServiceCredentialBinding.Create
func (m *MockServiceCredentialBinding) Create(ctx context.Context, r *resource.ServiceCredentialBindingCreate) (string, *resource.ServiceCredentialBinding, error) {
	args := m.Called()
	if len(args) == 2 {
		return args.String(0), nil, args.Error(1)
	}

	return args.String(0), args.Get(1).(*resource.ServiceCredentialBinding), args.Error(2)
}

// Delete mocks ServiceCredentialBinding.Delete
func (m *MockServiceCredentialBinding) Delete(ctx context.Context, guid string) (string, error) {
	args := m.Called(guid)
	return args.String(0), args.Error(1)
}

// ServiceCredentialBinding is a nil ServiceCredentialBinding
var (
	ServiceCredentialBindingNil *resource.ServiceCredentialBinding
)

// ServiceCredentialBinding is a ServiceCredentialBinding object
type ServiceCredentialBinding struct {
	resource.ServiceCredentialBinding
}

// NewServiceCredentialBindingDetails generate a new ServiceCredentialBindingDetails
func NewServiceCredentialBindingDetails(t string) *resource.ServiceCredentialBindingDetails {
	r := &resource.ServiceCredentialBindingDetails{}
	return r
}

// NewServiceInstance generate a new ServiceCredentialBinding
func NewServiceCredentialBinding(t string) *ServiceCredentialBinding {
	r := &ServiceCredentialBinding{}
	r.Type = t
	return r
}

// SetName assigns ServiceCredentialBinding name
func (s *ServiceCredentialBinding) SetName(name string) *ServiceCredentialBinding {
	s.Name = &name
	return s
}

// SetGUID assigns ServiceCredentialBinding GUID
func (s *ServiceCredentialBinding) SetGUID(guid string) *ServiceCredentialBinding {
	s.GUID = guid
	return s
}

// SetServiceInstanceRef assigns ServiceCredentialBinding ServiceInstanceRef
func (s *ServiceCredentialBinding) SetServiceInstanceRef(guid string) *ServiceCredentialBinding {
	s.Relationships = resource.ServiceCredentialBindingRelationships{ServiceInstance: &resource.ToOneRelationship{Data: &resource.Relationship{GUID: guid}}}
	return s
}

// SetLastOperation assigns ServiceCredentialBinding LastOperation
func (s *ServiceCredentialBinding) SetLastOperation(op, state string) *ServiceCredentialBinding {
	s.LastOperation = resource.LastOperation{
		Type:        op,
		State:       state,
		Description: op + " " + state,
		UpdatedAt:   testTime,
	}
	return s
}
