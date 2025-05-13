package fake

import (
	"context"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/stretchr/testify/mock"
)

// MockSpace mocks Space interfaces
type MockSpace struct {
	mock.Mock
}

// MockFeature mocks Feature interfaces
type MockFeature struct {
	mock.Mock
}

// EnableSSH mocks Feature.EnableSSH
func (m *MockFeature) EnableSSH(ctx context.Context, spaceGUID string, enable bool) error {
	args := m.Called()
	return args.Error(0)
}

// IsSSHEnabled mocks Feature.IsSSHEnabled
func (m *MockFeature) IsSSHEnabled(ctx context.Context, spaceGUID string) (bool, error) {
	args := m.Called()
	return args.Bool(0), args.Error(1)
}

// Get mocks Space.Get
func (m *MockSpace) Get(ctx context.Context, guid string) (*resource.Space, error) {
	args := m.Called(guid)
	return args.Get(0).(*resource.Space), args.Error(1)
}

// Single mocks Space.Single
func (m *MockSpace) Single(ctx context.Context, opts *client.SpaceListOptions) (*resource.Space, error) {
	args := m.Called()
	return args.Get(0).(*resource.Space), args.Error(1)
}

// Create mocks Space.Create
func (m *MockSpace) Create(ctx context.Context, r *resource.SpaceCreate) (*resource.Space, error) {
	args := m.Called()
	return args.Get(0).(*resource.Space), args.Error(1)
}

// Update mocks Space.Update
func (m *MockSpace) Update(ctx context.Context, guid string, opt *resource.SpaceUpdate) (*resource.Space, error) {
	args := m.Called()
	return args.Get(0).(*resource.Space), args.Error(1)
}

// Delete mocks Space.Delete
func (m *MockSpace) Delete(ctx context.Context, guid string) (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

// Space is a nil Space
var (
	SpaceNil *resource.Space
)

// Space is a Space object
type Space struct {
	resource.Space
}

// NewSpace generate a new Space
func NewSpace() *Space {
	r := &Space{}
	return r
}

// SetName assigns Space name
func (s *Space) SetName(name string) *Space {
	s.Name = name
	return s
}

// SetGUID assigns Space GUID
func (s *Space) SetGUID(guid string) *Space {
	s.GUID = guid
	return s
}

// SetRelationships assigns Space relationships
func (s *Space) SetRelationships(guid string) *Space {
	s.Relationships = &resource.SpaceRelationships{
		Organization: &resource.ToOneRelationship{
			Data: &resource.Relationship{
				GUID: guid,
			},
		},
	}
	return s
}
