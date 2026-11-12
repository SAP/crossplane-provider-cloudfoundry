package servicecredentialbinding

import (
	"context"
	"errors"
	"testing"

	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/fake"
)

func deleteTestBinding(guid, operationType, state string) *cfresource.ServiceCredentialBinding {
	return &cfresource.ServiceCredentialBinding{
		Resource:      cfresource.Resource{GUID: guid},
		LastOperation: cfresource.LastOperation{Type: operationType, State: state},
	}
}

func expectDeleteConfirmed(cf *fake.MockServiceCredentialBinding, guid string) {
	cf.On("Get", mock.Anything, guid).Return(deleteTestBinding(guid, "create", "succeeded"), nil).Once()
	cf.On("Delete", mock.Anything, guid).Return("", nil).Once()
	cf.On("Get", mock.Anything, guid).Return((*cfresource.ServiceCredentialBinding)(nil), cfresource.NewResourceNotFoundError()).Once()
}

func TestDeleteConfirmsBindingAbsence(t *testing.T) {
	const guid = "binding-guid"
	binding := func(operationType, state string) *cfresource.ServiceCredentialBinding {
		return deleteTestBinding(guid, operationType, state)
	}
	notFoundErrors := map[string]error{
		"resource not found": cfresource.NewResourceNotFoundError(),
		"binding not found":  cfresource.NewServiceBindingNotFoundError(),
	}

	t.Run("already absent", func(t *testing.T) {
		for name, notFound := range notFoundErrors {
			t.Run(name, func(t *testing.T) {
				cf := &fake.MockServiceCredentialBinding{}
				cf.On("Get", mock.Anything, guid).Return((*cfresource.ServiceCredentialBinding)(nil), notFound).Once()
				require.NoError(t, Delete(context.Background(), cf, guid))
				cf.AssertNotCalled(t, "Delete", mock.Anything, guid)
				cf.AssertExpectations(t)
			})
		}
	})

	t.Run("preflight failure retains identity", func(t *testing.T) {
		cf := &fake.MockServiceCredentialBinding{}
		cf.On("Get", mock.Anything, guid).Return((*cfresource.ServiceCredentialBinding)(nil), errors.New("lookup failed")).Once()
		require.Error(t, Delete(context.Background(), cf, guid))
		cf.AssertNotCalled(t, "Delete", mock.Anything, guid)
	})

	t.Run("nil preflight response is not absence", func(t *testing.T) {
		cf := &fake.MockServiceCredentialBinding{}
		cf.On("Get", mock.Anything, guid).Return((*cfresource.ServiceCredentialBinding)(nil), nil).Once()
		err := Delete(context.Background(), cf, guid)
		require.Error(t, err)
		require.NotErrorIs(t, err, ErrDeletionPending, "a nil API response is anomalous, not routine async progress")
		cf.AssertNotCalled(t, "Delete", mock.Anything, guid)
	})

	for _, state := range []string{"initial", "in progress", "succeeded"} {
		t.Run("delete operation "+state+" is not resubmitted", func(t *testing.T) {
			cf := &fake.MockServiceCredentialBinding{}
			cf.On("Get", mock.Anything, guid).Return(binding("delete", state), nil).Once()
			require.ErrorIs(t, Delete(context.Background(), cf, guid), ErrDeletionPending)
			cf.AssertNotCalled(t, "Delete", mock.Anything, guid)
		})
	}

	for _, state := range []string{"unknown", ""} {
		t.Run("unknown delete operation state "+state+" is diagnostic", func(t *testing.T) {
			cf := &fake.MockServiceCredentialBinding{}
			cf.On("Get", mock.Anything, guid).Return(binding("delete", state), nil).Once()
			err := Delete(context.Background(), cf, guid)
			require.Error(t, err)
			require.NotErrorIs(t, err, ErrDeletionPending)
			cf.AssertNotCalled(t, "Delete", mock.Anything, guid)
		})
	}

	t.Run("accepted delete and confirmed not found", func(t *testing.T) {
		for name, notFound := range notFoundErrors {
			t.Run(name, func(t *testing.T) {
				cf := &fake.MockServiceCredentialBinding{}
				cf.On("Get", mock.Anything, guid).Return(binding("create", "succeeded"), nil).Once()
				cf.On("Delete", mock.Anything, guid).Return("", nil).Once()
				cf.On("Get", mock.Anything, guid).Return((*cfresource.ServiceCredentialBinding)(nil), notFound).Once()
				require.NoError(t, Delete(context.Background(), cf, guid))
				cf.AssertExpectations(t)
			})
		}
	})

	t.Run("accepted asynchronous delete with binding still present is pending", func(t *testing.T) {
		cf := &fake.MockServiceCredentialBinding{}
		cf.On("Get", mock.Anything, guid).Return(binding("create", "succeeded"), nil).Once()
		cf.On("Delete", mock.Anything, guid).Return("job-guid", nil).Once()
		cf.On("Get", mock.Anything, guid).Return(binding("delete", "in progress"), nil).Once()
		require.ErrorIs(t, Delete(context.Background(), cf, guid), ErrDeletionPending)
		cf.AssertExpectations(t)
	})

	t.Run("empty job response does not establish synchronous completion", func(t *testing.T) {
		cf := &fake.MockServiceCredentialBinding{}
		cf.On("Get", mock.Anything, guid).Return(binding("create", "succeeded"), nil).Once()
		cf.On("Delete", mock.Anything, guid).Return("", nil).Once()
		cf.On("Get", mock.Anything, guid).Return(binding("create", "succeeded"), nil).Once()
		require.ErrorIs(t, Delete(context.Background(), cf, guid), ErrDeletionPending)
		cf.AssertExpectations(t)
	})

	t.Run("failed delete operation is retried", func(t *testing.T) {
		cf := &fake.MockServiceCredentialBinding{}
		cf.On("Get", mock.Anything, guid).Return(binding("delete", "failed"), nil).Once()
		cf.On("Delete", mock.Anything, guid).Return("", cfresource.NewServiceBindingNotFoundError()).Once()
		require.NoError(t, Delete(context.Background(), cf, guid))
		cf.AssertExpectations(t)
	})

	t.Run("delete request failure is not completion", func(t *testing.T) {
		cf := &fake.MockServiceCredentialBinding{}
		cf.On("Get", mock.Anything, guid).Return(binding("create", "succeeded"), nil).Once()
		cf.On("Delete", mock.Anything, guid).Return("", errors.New("request outcome unknown")).Once()
		require.Error(t, Delete(context.Background(), cf, guid))
		cf.AssertExpectations(t)
	})

	t.Run("post-delete lookup failure is not completion", func(t *testing.T) {
		cf := &fake.MockServiceCredentialBinding{}
		cf.On("Get", mock.Anything, guid).Return(binding("create", "succeeded"), nil).Once()
		cf.On("Delete", mock.Anything, guid).Return("", nil).Once()
		cf.On("Get", mock.Anything, guid).Return((*cfresource.ServiceCredentialBinding)(nil), errors.New("lookup failed")).Once()
		require.Error(t, Delete(context.Background(), cf, guid))
		cf.AssertExpectations(t)
	})

	t.Run("nil post-delete response is not absence", func(t *testing.T) {
		cf := &fake.MockServiceCredentialBinding{}
		cf.On("Get", mock.Anything, guid).Return(binding("create", "succeeded"), nil).Once()
		cf.On("Delete", mock.Anything, guid).Return("", nil).Once()
		cf.On("Get", mock.Anything, guid).Return((*cfresource.ServiceCredentialBinding)(nil), nil).Once()
		err := Delete(context.Background(), cf, guid)
		require.Error(t, err)
		require.NotErrorIs(t, err, ErrDeletionPending, "a nil confirmation response is anomalous")
		cf.AssertExpectations(t)
	})

	t.Run("cancellation after accepted delete is incomplete", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		cf := &fake.MockServiceCredentialBinding{}
		cf.On("Get", mock.Anything, guid).Return(binding("create", "succeeded"), nil).Once()
		cf.On("Delete", mock.Anything, guid).Return("job-guid", nil).Run(func(mock.Arguments) { cancel() }).Once()
		require.ErrorIs(t, Delete(ctx, cf, guid), context.Canceled)
		cf.AssertNumberOfCalls(t, "Get", 1)
	})

	t.Run("canceled context does not issue requests", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		cf := &fake.MockServiceCredentialBinding{}
		require.ErrorIs(t, Delete(ctx, cf, guid), context.Canceled)
		cf.AssertNotCalled(t, "Get", mock.Anything, guid)
		cf.AssertNotCalled(t, "Delete", mock.Anything, guid)
	})
}
