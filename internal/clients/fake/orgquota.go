package fake

import (
	"context"

	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/stretchr/testify/mock"
)

// MockOrgQuota mocks OrgQuota interfaces
type MockOrgQuota struct {
	mock.Mock
}

func (m *MockOrgQuota) Get(ctx context.Context, guid string) (*resource.OrganizationQuota, error) {
	args := m.Called(guid)
	return args.Get(0).(*resource.OrganizationQuota), args.Error(1)
}

func (m *MockOrgQuota) Create(ctx context.Context, opt *resource.OrganizationQuotaCreateOrUpdate) (*resource.OrganizationQuota, error) {
	args := m.Called()
	return args.Get(0).(*resource.OrganizationQuota), args.Error(1)
}

func (m *MockOrgQuota) Update(ctx context.Context, guid string, opt *resource.OrganizationQuotaCreateOrUpdate) (*resource.OrganizationQuota, error) {
	args := m.Called()
	return args.Get(0).(*resource.OrganizationQuota), args.Error(1)
}

func (m *MockOrgQuota) Delete(ctx context.Context, guid string) (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}
