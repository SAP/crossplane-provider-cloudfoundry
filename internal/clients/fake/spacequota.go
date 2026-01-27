package fake

import (
	"context"

	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/stretchr/testify/mock"
)

// MockSpaceQuota mocks SpaceQuota interfaces
type MockSpaceQuota struct {
	mock.Mock
}

func (m *MockSpaceQuota) Get(ctx context.Context, guid string) (*resource.SpaceQuota, error) {
	args := m.Called(guid)
	return args.Get(0).(*resource.SpaceQuota), args.Error(1)
}

func (m *MockSpaceQuota) Create(ctx context.Context, r *resource.SpaceQuotaCreateOrUpdate) (*resource.SpaceQuota, error) {
	args := m.Called()
	return args.Get(0).(*resource.SpaceQuota), args.Error(1)
}

func (m *MockSpaceQuota) Update(ctx context.Context, guid string, r *resource.SpaceQuotaCreateOrUpdate) (*resource.SpaceQuota, error) {
	args := m.Called()
	return args.Get(0).(*resource.SpaceQuota), args.Error(1)
}

func (m *MockSpaceQuota) Apply(ctx context.Context, guid string, spaceGUIDs []string) ([]string, error) {
	args := m.Called()
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockSpaceQuota) Remove(ctx context.Context, guid, spaceGUID string) error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockSpaceQuota) Delete(ctx context.Context, guid string) (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

// SpaceQuotaNil is a nil SpaceQuota
var (
	SpaceQuotaNil *resource.SpaceQuota
)

// SpaceQuota is a CloudFoundry SpaceQuota object
type SpaceQuota struct {
	resource.SpaceQuota
}

// NewSpaceQuota generates a new SpaceQuota
func NewSpaceQuota() *SpaceQuota {
	return &SpaceQuota{}
}

// SetName assigns Space name
func (s *SpaceQuota) SetName(name string) *SpaceQuota {
	s.Name = name
	return s
}

// SetGUID assigns SpaceQuota GUID
func (s *SpaceQuota) SetGUID(guid string) *SpaceQuota {
	s.GUID = guid
	return s
}

// SetOrgGUID assigns Space relationships
func (s *SpaceQuota) SetOrgGUID(guid string) *SpaceQuota {
	s.Relationships = resource.SpaceQuotaRelationships{
		Organization: &resource.ToOneRelationship{
			Data: &resource.Relationship{
				GUID: guid,
			},
		},
	}
	return s
}
