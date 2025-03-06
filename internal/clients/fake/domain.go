package fake

import (
	"context"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/stretchr/testify/mock"
)

// MockDomain mocks Domain interfaces
type MockDomain struct {
	mock.Mock
}

// Get mocks Domain.Get
func (m *MockDomain) Get(ctx context.Context, guid string) (*resource.Domain, error) {
	args := m.Called(guid)
	return args.Get(0).(*resource.Domain), args.Error(1)
}

// Single mocks Domain.Single
func (m *MockDomain) Single(ctx context.Context, opt *client.DomainListOptions) (*resource.Domain, error) {
	args := m.Called()
	return args.Get(0).(*resource.Domain), args.Error(1)
}

// Create mocks Domain.Create
func (m *MockDomain) Create(ctx context.Context, opt *resource.DomainCreate) (*resource.Domain, error) {
	args := m.Called()
	return args.Get(0).(*resource.Domain), args.Error(1)
}

// Update mocks Domain.Update
func (m *MockDomain) Update(ctx context.Context, guid string, opt *resource.DomainUpdate) (*resource.Domain, error) {
	args := m.Called()
	return args.Get(0).(*resource.Domain), args.Error(1)
}

// Delete mocks Domain.Delete
func (m *MockDomain) Delete(ctx context.Context, guid string) (string, error) {
	args := m.Called()
	return args.Get(0).(string), args.Error(1)
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
