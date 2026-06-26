package fake

import (
	"context"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/stretchr/testify/mock"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
)

// MockDomain mocks Domain interfaces
type MockDomain struct {
	mock.Mock
}

// Get mocks Domain.Get
func (m *MockDomain) Get(ctx context.Context, guid string) (*resource.Domain, error) {
	args := m.Called(guid)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*resource.Domain), args.Error(1)
}

// Single mocks Domain.Single
func (m *MockDomain) Single(ctx context.Context, opt *client.DomainListOptions) (*resource.Domain, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*resource.Domain), args.Error(1)
}

// Create mocks Domain.Create
func (m *MockDomain) Create(ctx context.Context, opt *resource.DomainCreate) (*resource.Domain, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*resource.Domain), args.Error(1)
}

// Update mocks Domain.Update
func (m *MockDomain) Update(ctx context.Context, guid string, opt *resource.DomainUpdate) (*resource.Domain, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*resource.Domain), args.Error(1)
}

// Delete mocks Domain.Delete
func (m *MockDomain) Delete(ctx context.Context, guid string) (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

// FindDomainBySpec mocks Domain.FindDomainBySpec
func (m *MockDomain) FindDomainBySpec(ctx context.Context, spec v1alpha1.DomainParameters) (*resource.Domain, error) {
	args := m.Called(ctx, spec)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*resource.Domain), args.Error(1)
}

// GetDomainByGUID mocks Domain.GetDomainByGUID
func (m *MockDomain) GetDomainByGUID(ctx context.Context, guid string) (*resource.Domain, error) {
	args := m.Called(ctx, guid)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*resource.Domain), args.Error(1)
}

// Domain is a nil Domain
var (
	DomainNil *resource.Domain
)

// Domain is a Domain object
type Domain struct {
	resource.Domain
}

// NewDomain generate a new Domain
func NewDomain() *Domain {
	r := &Domain{}
	return r
}

// SetName assigns Domain name
func (s *Domain) SetName(name string) *Domain {
	s.Name = name
	return s
}

// SetGUID assigns Domain GUID
func (s *Domain) SetGUID(guid string) *Domain {
	s.GUID = guid
	return s
}

func (s *Domain) SetLabels(labels map[string]*string) *Domain {
	if s.Metadata == nil {
		s.Metadata = &resource.Metadata{}
	}
	s.Metadata.Labels = labels
	return s
}
func (s *Domain) SetAnnotations(annotations map[string]*string) *Domain {
	if s.Metadata == nil {
		s.Metadata = &resource.Metadata{}
	}
	s.Metadata.Annotations = annotations
	return s
}
