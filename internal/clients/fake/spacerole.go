package fake

import (
	"context"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/stretchr/testify/mock"
)

// MockSpaceRole mocks SpaceRole interfaces
type MockSpaceRole struct {
	mock.Mock
}

// Get mocks SpaceRole.Get
func (m *MockSpaceRole) Get(ctx context.Context, guid string) (*resource.Role, error) {
	args := m.Called(guid)
	return args.Get(0).(*resource.Role), args.Error(1)
}

// Single mocks SpaceRole.Single
func (m *MockSpaceRole) Single(ctx context.Context, opt *client.RoleListOptions) (*resource.Role, error) {
	args := m.Called()
	return args.Get(0).(*resource.Role), args.Error(1)
}

// CreateOrganizationRoleWithUsername mocks SpaceRole.CreateSpaceRoleWithUsername
func (m *MockSpaceRole) CreateOrganizationRoleWithUsername(context.Context, string, string, resource.OrganizationRoleType, string) (*resource.Role, error) {
	args := m.Called()

	var emptyRole *resource.Role
	if args.Get(0) == emptyRole {
		return emptyRole, args.Error(1)
	}
	return args.Get(0).(*resource.Role), args.Error(1)
}

// CreateSpaceRoleWithUsername mocks SpaceRole.CreateSpaceRoleWithUsername
func (m *MockSpaceRole) CreateSpaceRoleWithUsername(context.Context, string, string, resource.SpaceRoleType, string) (*resource.Role, error) {
	args := m.Called()

	var emptyRole *resource.Role
	if args.Get(0) == emptyRole {
		return emptyRole, args.Error(1)
	}
	return args.Get(0).(*resource.Role), args.Error(1)
}

// Delete mocks SpaceRole.Delete
func (m *MockSpaceRole) Delete(context.Context, string) (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

// ListIncludeUsersAll mocks SpaceRole.ListIncludeUsersAll
func (m *MockSpaceRole) ListIncludeUsersAll(ctx context.Context, opts *client.RoleListOptions) ([]*resource.Role, []*resource.User, error) {
	args := m.Called()
	return args.Get(0).([]*resource.Role), args.Get(1).([]*resource.User), args.Error(2)
}

// SpaceRoleNil is a nil SpaceRole
var (
	SpaceRoleNil *resource.Role
)

// SpaceRole is a SpaceRole object
type SpaceRole struct {
	resource.Role
}

// NewSpaceRole generates a new SpaceRole
func NewSpaceRole() *SpaceRole {
	r := &SpaceRole{}
	return r
}

// SetType sets the type of the SpaceRole
func (o *SpaceRole) SetType(roleType string) *SpaceRole {
	o.Type = roleType
	return o
}

// SetGUID assigns SpaceRole GUID
func (o *SpaceRole) SetGUID(guid string) *SpaceRole {
	o.GUID = guid
	return o
}

// SetRelationships sets the relationships of the SpaceRole
func (o *SpaceRole) SetRelationships(relationships resource.RoleSpaceUserOrganizationRelationships) *SpaceRole {
	o.Relationships = relationships
	return o
}
