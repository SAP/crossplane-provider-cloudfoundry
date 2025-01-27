package fake

import (
	"context"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/stretchr/testify/mock"
)

// MockOrganization mocks Organization interfaces
type MockOrganization struct {
	mock.Mock
}

// Get mocks Organization.Get
func (m *MockOrganization) Get(ctx context.Context, guid string) (*resource.Organization, error) {
	args := m.Called(guid)
	return args.Get(0).(*resource.Organization), args.Error(1)
}

// Single mocks Organization.Single
func (m *MockOrganization) Single(ctx context.Context, opt *client.OrganizationListOptions) (*resource.Organization, error) {
	args := m.Called()
	return args.Get(0).(*resource.Organization), args.Error(1)
}

// Create mocks Organization.CreateManaged
func (m *MockOrganization) Create(ctx context.Context, opt *resource.OrganizationCreate) (*resource.Organization, error) {
	args := m.Called()
	return args.Get(0).(*resource.Organization), args.Error(1)
}

// Organization is a nil Organization
var (
	OrganizationNil *resource.Organization
)

// Organization is a Organization object
type Organization struct {
	resource.Organization
}

// NewOrganization generate a new Organization
func NewOrganization() *Organization {
	r := &Organization{}
	return r
}

// SetName assigns Organization name
func (s *Organization) SetName(name string) *Organization {
	s.Name = name
	return s
}

// SetGUID assigns Organization GUID
func (s *Organization) SetGUID(guid string) *Organization {
	s.GUID = guid
	return s
}
