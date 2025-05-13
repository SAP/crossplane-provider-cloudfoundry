package fake

import (
	"context"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/stretchr/testify/mock"
)

// MockOrgRole mocks OrgRole interfaces
type MockOrgRole struct {
	mock.Mock
}

// Get mocks OrgRole.Get
func (m *MockOrgRole) Get(ctx context.Context, guid string) (*resource.Role, error) {
	args := m.Called(guid)
	return args.Get(0).(*resource.Role), args.Error(1)
}

// Single mocks OrgRole.Single
func (m *MockOrgRole) Single(ctx context.Context, opt *client.RoleListOptions) (*resource.Role, error) {
	args := m.Called()
	return args.Get(0).(*resource.Role), args.Error(1)
}

// CreateOrganizationRoleWithUsername mocks OrgRole.CreateSpaceRoleWithUsername
func (m *MockOrgRole) CreateOrganizationRoleWithUsername(context.Context, string, string, resource.OrganizationRoleType, string) (*resource.Role, error) {
	args := m.Called()

	var emptyRole *resource.Role
	if args.Get(0) == emptyRole {
		return emptyRole, args.Error(1)
	}
	return args.Get(0).(*resource.Role), args.Error(1)
}

// CreateSpaceRoleWithUsername mocks OrgRole.CreateSpaceRoleWithUsername
func (m *MockOrgRole) CreateSpaceRoleWithUsername(context.Context, string, string, resource.SpaceRoleType, string) (*resource.Role, error) {
	args := m.Called()
	return args.Get(0).(*resource.Role), args.Error(1)
}

// Delete mocks OrgRole.Delete
func (m *MockOrgRole) Delete(context.Context, string) (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

// ListIncludeUsersAll mocks OrgRole.ListIncludeUsersAll
func (m *MockOrgRole) ListIncludeUsersAll(ctx context.Context, opts *client.RoleListOptions) ([]*resource.Role, []*resource.User, error) {
	args := m.Called()
	return args.Get(0).([]*resource.Role), args.Get(1).([]*resource.User), args.Error(2)
}

// OrganizationRoleNil is a nil OrgRole
var (
	OrganizationRoleNil *resource.Role
)

// OrgRole is a OrgRole object
type OrgRole struct {
	resource.Role
}

// NewOrgRole generates a new OrgRole
func NewOrgRole() *OrgRole {
	r := &OrgRole{}
	return r
}

// SetType sets the type of the OrgRole
func (o *OrgRole) SetType(roleType string) *OrgRole {
	o.Type = roleType
	return o
}

// SetGUID assigns OrgRole GUID
func (o *OrgRole) SetGUID(guid string) *OrgRole {
	o.GUID = guid
	return o
}

// SetRelationships sets the relationships of the OrgRole
func (o *OrgRole) SetRelationships(relationships resource.RoleSpaceUserOrganizationRelationships) *OrgRole {
	o.Relationships = relationships
	return o
}
