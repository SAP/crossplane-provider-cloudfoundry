package fake

import (
	"context"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/stretchr/testify/mock"
)

// MockServiceRouteBinding mocks ServiceRouteBinding interfaces
type MockServiceRouteBinding struct {
	mock.Mock
}

// Get mocks ServiceRouteBinding.Get
func (m *MockServiceRouteBinding) Get(ctx context.Context, guid string) (*resource.ServiceRouteBinding, error) {
	args := m.Called(ctx, guid)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*resource.ServiceRouteBinding), args.Error(1)
}

// PollComplete mocks Job.PollComplete
func (m *MockServiceRouteBinding) PollComplete(ctx context.Context, jobGUID string, opt *client.PollingOptions) error {
	args := m.Called(ctx, jobGUID, opt)
	return args.Error(0)
}

// GetParameters mocks ServiceRouteBinding.GetParameters
func (m *MockServiceRouteBinding) GetParameters(ctx context.Context, guid string) (map[string]string, error) {
	args := m.Called(ctx, guid)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]string), args.Error(1)
}

// Update mocks ServiceRouteBinding.Update
func (m *MockServiceRouteBinding) Update(ctx context.Context, guid string, r *resource.ServiceRouteBindingUpdate) (*resource.ServiceRouteBinding, error) {
	args := m.Called(ctx, guid, r)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*resource.ServiceRouteBinding), args.Error(1)
}

// Single mocks ServiceRouteBinding.Single
func (m *MockServiceRouteBinding) Single(ctx context.Context, opt *client.ServiceRouteBindingListOptions) (*resource.ServiceRouteBinding, error) {
	args := m.Called(ctx, opt)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*resource.ServiceRouteBinding), args.Error(1)
}

// Create mocks ServiceRouteBinding.Create
func (m *MockServiceRouteBinding) Create(ctx context.Context, r *resource.ServiceRouteBindingCreate) (string, *resource.ServiceRouteBinding, error) {
	args := m.Called(ctx, r)
	if args.Get(1) == nil {
		return args.String(0), nil, args.Error(2)
	}
	return args.String(0), args.Get(1).(*resource.ServiceRouteBinding), args.Error(2)
}

// Delete mocks ServiceRouteBinding.Delete
func (m *MockServiceRouteBinding) Delete(ctx context.Context, guid string) (string, error) {
	args := m.Called(ctx, guid)
	return args.String(0), args.Error(1)
}

// ServiceRouteBinding is a nil ServiceRouteBinding
var (
	ServiceRouteBindingNil *resource.ServiceRouteBinding
)

// ServiceRouteBinding is a ServiceRouteBinding object
type ServiceRouteBinding struct {
	resource.ServiceRouteBinding
}

// NewServiceRouteBinding generate a new ServiceRouteBinding
func NewServiceRouteBinding() *ServiceRouteBinding {
	r := &ServiceRouteBinding{}
	r.CreatedAt = testTime
	r.UpdatedAt = testTime
	return r
}

// SetGUID assigns ServiceRouteBinding GUID
func (s *ServiceRouteBinding) SetGUID(guid string) *ServiceRouteBinding {
	s.GUID = guid
	return s
}

// SetRouteRef assigns ServiceRouteBinding RouteRef
func (s *ServiceRouteBinding) SetRouteRef(guid string) *ServiceRouteBinding {
	if s.Relationships.Route.Data == nil {
		s.Relationships.Route.Data = &resource.Relationship{}
	}
	s.Relationships.Route.Data.GUID = guid
	return s
}

// SetServiceInstanceRef assigns ServiceRouteBinding ServiceInstanceRef
func (s *ServiceRouteBinding) SetServiceInstanceRef(guid string) *ServiceRouteBinding {
	if s.Relationships.ServiceInstance.Data == nil {
		s.Relationships.ServiceInstance.Data = &resource.Relationship{}
	}
	s.Relationships.ServiceInstance.Data.GUID = guid
	return s
}

// SetRouteServiceURL assigns ServiceRouteBinding RouteServiceURL
func (s *ServiceRouteBinding) SetRouteServiceURL(url string) *ServiceRouteBinding {
	s.RouteServiceURL = url
	return s
}

// SetLastOperation assigns ServiceRouteBinding LastOperation
func (s *ServiceRouteBinding) SetLastOperation(op, state string) *ServiceRouteBinding {
	s.LastOperation = resource.LastOperation{
		Type:        op,
		State:       state,
		Description: op + " " + state,
		UpdatedAt:   testTime,
		CreatedAt:   testTime,
	}
	return s
}

// SetLabels assigns ServiceRouteBinding Labels
func (s *ServiceRouteBinding) SetLabels(labels map[string]*string) *ServiceRouteBinding {
	if s.Metadata == nil {
		s.Metadata = &resource.Metadata{}
	}
	s.Metadata.Labels = labels
	return s
}

// SetAnnotations assigns ServiceRouteBinding Annotations
func (s *ServiceRouteBinding) SetAnnotations(annotations map[string]*string) *ServiceRouteBinding {
	if s.Metadata == nil {
		s.Metadata = &resource.Metadata{}
	}
	s.Metadata.Annotations = annotations
	return s
}
