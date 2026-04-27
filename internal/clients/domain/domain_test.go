package domain

import (
	"context"
	"testing"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
)

// MockClient implements the Client interface for testing
type MockClient struct {
	mock.Mock
}

func (m *MockClient) Get(ctx context.Context, guid string) (*resource.Domain, error) {
	args := m.Called(ctx, guid)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*resource.Domain), args.Error(1)
}

func (m *MockClient) Single(ctx context.Context, opt *client.DomainListOptions) (*resource.Domain, error) {
	args := m.Called(ctx, opt)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*resource.Domain), args.Error(1)
}

func (m *MockClient) Create(ctx context.Context, create *resource.DomainCreate) (*resource.Domain, error) {
	args := m.Called(ctx, create)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*resource.Domain), args.Error(1)
}

func (m *MockClient) Delete(ctx context.Context, guid string) (string, error) {
	args := m.Called(ctx, guid)
	return args.String(0), args.Error(1)
}

func (m *MockClient) Update(ctx context.Context, guid string, update *resource.DomainUpdate) (*resource.Domain, error) {
	args := m.Called(ctx, guid, update)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*resource.Domain), args.Error(1)
}

func TestClientWrapper_FindDomainBySpec(t *testing.T) {
	ctx := context.Background()
	mockClient := new(MockClient)

	spec := v1alpha1.DomainParameters{
		Name: "test-domain.com",
	}

	expectedDomain := &resource.Domain{
		Resource: resource.Resource{
			GUID: "test-guid-123",
		},
		Name: "test-domain.com",
	}

	mockClient.On("Single", ctx, &client.DomainListOptions{
		Names: client.Filter{Values: []string{"test-domain.com"}},
	}).Return(expectedDomain, nil)

	wrapper := &ClientWrapper{Client: mockClient}
	result, err := wrapper.FindDomainBySpec(ctx, spec)
	require.NoError(t, err)
	assert.Equal(t, expectedDomain, result)
	mockClient.AssertExpectations(t)
}

func TestClientWrapper_FindDomainBySpec_WithOrg(t *testing.T) {
	ctx := context.Background()
	mockClient := new(MockClient)

	orgGUID := "org-guid-789"
	spec := v1alpha1.DomainParameters{
		Name:         "test-domain.com",
		OrgReference: v1alpha1.OrgReference{Org: &orgGUID},
	}

	expectedDomain := &resource.Domain{
		Resource: resource.Resource{
			GUID: "test-guid-123",
		},
		Name: "test-domain.com",
	}

	mockClient.On("Single", ctx, &client.DomainListOptions{
		Names:             client.Filter{Values: []string{"test-domain.com"}},
		OrganizationGUIDs: client.Filter{Values: []string{"org-guid-789"}},
	}).Return(expectedDomain, nil)

	wrapper := &ClientWrapper{Client: mockClient}
	result, err := wrapper.FindDomainBySpec(ctx, spec)
	require.NoError(t, err)
	assert.Equal(t, expectedDomain, result)
	mockClient.AssertExpectations(t)
}

func TestClientWrapper_FindDomainBySpec_NotFound(t *testing.T) {
	ctx := context.Background()
	mockClient := new(MockClient)

	spec := v1alpha1.DomainParameters{
		Name: "nonexistent-domain.com",
	}

	mockClient.On("Single", ctx, &client.DomainListOptions{
		Names: client.Filter{Values: []string{"nonexistent-domain.com"}},
	}).Return(nil, assert.AnError)

	wrapper := &ClientWrapper{Client: mockClient}
	result, err := wrapper.FindDomainBySpec(ctx, spec)
	require.Error(t, err)
	assert.Nil(t, result)
	mockClient.AssertExpectations(t)
}

func TestClientWrapper_GetDomainByGUID(t *testing.T) {
	ctx := context.Background()
	mockClient := new(MockClient)

	guid := "test-guid-456"
	expectedDomain := &resource.Domain{
		Resource: resource.Resource{
			GUID: guid,
		},
		Name: "test-domain.com",
	}

	mockClient.On("Get", ctx, guid).Return(expectedDomain, nil)

	wrapper := &ClientWrapper{Client: mockClient}
	result, err := wrapper.GetDomainByGUID(ctx, guid)
	require.NoError(t, err)
	assert.Equal(t, expectedDomain, result)
	mockClient.AssertExpectations(t)
}

func TestClientWrapper_GetDomainByGUID_NotFound(t *testing.T) {
	ctx := context.Background()
	mockClient := new(MockClient)

	guid := "nonexistent-guid"

	mockClient.On("Get", ctx, guid).Return(nil, assert.AnError)

	wrapper := &ClientWrapper{Client: mockClient}
	result, err := wrapper.GetDomainByGUID(ctx, guid)
	require.Error(t, err)
	assert.Nil(t, result)
	mockClient.AssertExpectations(t)
}
