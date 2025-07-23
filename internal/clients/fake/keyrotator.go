package fake

import (
	"context"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/stretchr/testify/mock"
)

type MockKeyRotator struct {
	mock.Mock
}

func (m *MockKeyRotator) RetireBinding(cr *v1alpha1.ServiceCredentialBinding, serviceBinding *cfresource.ServiceCredentialBinding) bool {
	args := m.Called(cr, serviceBinding)
	return args.Bool(0)
}

func (m *MockKeyRotator) HasExpiredKeys(cr *v1alpha1.ServiceCredentialBinding) bool {
	args := m.Called(cr)
	return args.Bool(0)
}

func (m *MockKeyRotator) DeleteExpiredKeys(ctx context.Context, cr *v1alpha1.ServiceCredentialBinding) ([]*v1alpha1.SCBResource, error) {
	args := m.Called(ctx, cr)
	if len(args) == 2 {
		return args.Get(0).([]*v1alpha1.SCBResource), args.Error(1)
	}
	return nil, args.Error(0)
}

func (m *MockKeyRotator) DeleteRetiredKeys(ctx context.Context, cr *v1alpha1.ServiceCredentialBinding) error {
	args := m.Called(ctx, cr)
	return args.Error(0)
}
