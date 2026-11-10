package servicecredentialbinding

import (
	"context"
	"errors"
	"testing"
	"time"

	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/fake"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/servicecredentialbinding"
)

func TestRotationCleanupNilSafety(t *testing.T) {
	retired := &v1alpha1.SCBResource{
		GUID:      rotationTestK0,
		CreatedAt: &metav1.Time{Time: time.Now().Add(-48 * time.Hour)},
	}
	cases := map[string]struct {
		rotation *v1alpha1.RotationParameters
		retired  []*v1alpha1.SCBResource
	}{
		"rotation absent": {retired: []*v1alpha1.SCBResource{retired}},
		"ttl absent": {
			rotation: &v1alpha1.RotationParameters{Frequency: &metav1.Duration{Duration: time.Hour}},
			retired:  []*v1alpha1.SCBResource{retired},
		},
		"nil entry":         {retired: []*v1alpha1.SCBResource{nil}},
		"missing timestamp": {retired: []*v1alpha1.SCBResource{{GUID: rotationTestK0}}},
		"empty identity":    {retired: []*v1alpha1.SCBResource{{CreatedAt: retired.CreatedAt}}},
		"empty list":        {retired: []*v1alpha1.SCBResource{}},
		"nil list":          {retired: nil},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cr := serviceCredentialBinding("key", withServiceInstanceID(serviceInstanceGUID), withExternalName(rotationTestK1))
			cr.Spec.ForProvider.Rotation = tc.rotation
			cr.Status.AtProvider.RetiredKeys = tc.retired
			cf := &fake.MockServiceCredentialBinding{}
			cf.On("Update", mock.Anything, rotationTestK1, mock.Anything).Return(
				&cfresource.ServiceCredentialBinding{Resource: cfresource.Resource{GUID: rotationTestK1}}, nil,
			)
			cf.On("Get", mock.Anything, rotationTestK1).Return(rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationSucceeded), nil).Maybe()
			rotator := &servicecredentialbinding.SCBKeyRotator{SCBClient: cf}
			ext := &external{
				kube: &test.MockClient{
					MockUpdate:       test.NewMockUpdateFn(nil),
					MockStatusUpdate: func(context.Context, k8s.Object, ...k8s.SubResourceUpdateOption) error { return nil },
				},
				scbClient:  cf,
				keyRotator: rotator,
			}

			assert.NotPanics(t, func() {
				_, err := ext.Update(context.Background(), cr)
				assert.NoError(t, err)
			})
			cf.AssertNotCalled(t, "Delete", mock.Anything, "")
			cf.AssertExpectations(t)
		})
	}
}

func TestRotationCleanupPartialProgress(t *testing.T) {
	removed := &v1alpha1.SCBResource{GUID: rotationTestK0, CreatedAt: &metav1.Time{Time: time.Now().Add(-48 * time.Hour)}}
	failed := &v1alpha1.SCBResource{GUID: "33333333-3333-4333-8333-333333333333", CreatedAt: &metav1.Time{Time: time.Now().Add(-48 * time.Hour)}}
	cr := serviceCredentialBinding("key", withServiceInstanceID(serviceInstanceGUID), withExternalName(rotationTestK1))
	cr.Spec.ForProvider.Rotation = &v1alpha1.RotationParameters{TTL: &metav1.Duration{Duration: time.Hour}}
	cr.Status.AtProvider.RetiredKeys = []*v1alpha1.SCBResource{removed, failed}
	cf := &fake.MockServiceCredentialBinding{}
	cf.On("Update", mock.Anything, rotationTestK1, mock.Anything).Return(
		&cfresource.ServiceCredentialBinding{Resource: cfresource.Resource{GUID: rotationTestK1}}, nil,
	)
	cf.On("Get", mock.Anything, rotationTestK1).Return(rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationSucceeded), nil)
	cf.On("Get", mock.Anything, removed.GUID).Return(rotationControllerBinding(removed.GUID, v1alpha1.LastOperationSucceeded), nil).Once()
	cf.On("Delete", mock.Anything, removed.GUID).Return("", nil).Once()
	cf.On("Get", mock.Anything, removed.GUID).Return((*cfresource.ServiceCredentialBinding)(nil), cfresource.NewResourceNotFoundError()).Once()
	pending := rotationControllerBinding(failed.GUID, v1alpha1.LastOperationInProgress)
	pending.LastOperation.Type = "delete"
	cf.On("Get", mock.Anything, failed.GUID).Return(pending, nil).Once()
	store := newRotationStoredClient(cr)
	ext := &external{
		kube:       store,
		scbClient:  cf,
		keyRotator: &servicecredentialbinding.SCBKeyRotator{SCBClient: cf},
	}

	_, err := ext.Update(context.Background(), cr)
	require.ErrorIs(t, err, servicecredentialbinding.ErrDeletionPending)
	require.ErrorContains(t, err, "cannot delete expired key "+failed.GUID)
	assert.Equal(t, []*v1alpha1.SCBResource{failed}, cr.Status.AtProvider.RetiredKeys,
		"successfully deleted and not-found keys should be removed even when another deletion fails")
	assert.Equal(t, []*v1alpha1.SCBResource{failed}, store.get().Status.AtProvider.RetiredKeys,
		"successful deletion progress must be persisted even when another deletion fails")
	cf.AssertExpectations(t)
}

func TestRotationCleanupAsyncDeletionRetries(t *testing.T) {
	for _, fallback := range []bool{false, true} {
		name := "explicit TTL"
		if fallback {
			name = "fallback deadline"
		}
		t.Run(name, func(t *testing.T) {
			now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
			retired := &v1alpha1.SCBResource{GUID: rotationTestK0}
			cr := serviceCredentialBinding("key", withServiceInstanceID(serviceInstanceGUID), withExternalName(rotationTestK1))
			cr.Status.AtProvider.GUID = rotationTestK1
			cr.Status.AtProvider.CreatedAt = &metav1.Time{Time: now.Add(-time.Hour)}
			if fallback {
				retired.DeleteAfter = &metav1.Time{Time: now.Add(-time.Second)}
			} else {
				cr.Spec.ForProvider.Rotation = &v1alpha1.RotationParameters{TTL: &metav1.Duration{Duration: time.Hour}}
				retired.CreatedAt = &metav1.Time{Time: now.Add(-2 * time.Hour)}
			}
			cr.Status.AtProvider.RetiredKeys = []*v1alpha1.SCBResource{retired}
			store := newRotationStoredClient(cr)
			cf := &fake.MockServiceCredentialBinding{}
			active := rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationSucceeded)
			cf.On("Update", mock.Anything, rotationTestK1, mock.Anything).Return(active, nil).Times(4)
			cf.On("Get", mock.Anything, rotationTestK1).Return(active, nil).Times(4)

			// The first reconcile accepts the async delete but retains the exact record.
			cf.On("Get", mock.Anything, rotationTestK0).Return(rotationControllerBinding(rotationTestK0, v1alpha1.LastOperationSucceeded), nil).Once()
			cf.On("Delete", mock.Anything, rotationTestK0).Return("delete-job", nil).Once()
			cf.On("Get", mock.Anything, rotationTestK0).Return(rotationControllerBinding(rotationTestK0, v1alpha1.LastOperationInProgress), nil).Once()
			assertRotationCleanupReconcile(t, store, cf, now)
			stored := store.get()
			require.Len(t, stored.Status.AtProvider.RetiredKeys, 1)
			require.Equal(t, retired, stored.Status.AtProvider.RetiredKeys[0])

			// A fresh object after restart observes delete-in-progress and must not resubmit.
			inProgress := rotationControllerBinding(rotationTestK0, v1alpha1.LastOperationInProgress)
			inProgress.LastOperation.Type = "delete"
			cf.On("Get", mock.Anything, rotationTestK0).Return(inProgress, nil).Once()
			assertRotationCleanupReconcile(t, store, cf, now)
			require.Equal(t, retired, store.get().Status.AtProvider.RetiredKeys[0])

			// Once the broker reports a failed unbind, retry exactly once; still-present
			// bindings remain tracked even when Cloud Foundry accepts the retry.
			failed := rotationControllerBinding(rotationTestK0, v1alpha1.LastOperationFailed)
			failed.LastOperation.Type = "delete"
			cf.On("Get", mock.Anything, rotationTestK0).Return(failed, nil).Once()
			cf.On("Delete", mock.Anything, rotationTestK0).Return("retry-job", nil).Once()
			cf.On("Get", mock.Anything, rotationTestK0).Return(rotationControllerBinding(rotationTestK0, v1alpha1.LastOperationInProgress), nil).Once()
			assertRotationCleanupReconcile(t, store, cf, now)
			require.Equal(t, retired, store.get().Status.AtProvider.RetiredKeys[0])

			// Confirmed absence is the only transition that removes the durable identity.
			cf.On("Get", mock.Anything, rotationTestK0).Return((*cfresource.ServiceCredentialBinding)(nil), cfresource.NewResourceNotFoundError()).Once()
			_, err := rotationCleanupUpdate(t, store, cf, now)
			require.NoError(t, err, "confirmed absence should remove the durable identity")
			stored = store.get()
			require.Empty(t, stored.Status.AtProvider.RetiredKeys)
			require.Equal(t, rotationTestK1, stored.Status.AtProvider.GUID)
			require.Equal(t, rotationTestK1, stored.GetAnnotations()["crossplane.io/external-name"])
			cf.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
			cf.AssertExpectations(t)
		})
	}
}

func TestRotationCleanupAsyncDeletionStatusWriteRetry(t *testing.T) {
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	retired := &v1alpha1.SCBResource{GUID: rotationTestK0, DeleteAfter: &metav1.Time{Time: now.Add(-time.Second)}}
	cr := serviceCredentialBinding("key", withServiceInstanceID(serviceInstanceGUID), withExternalName(rotationTestK1))
	cr.Status.AtProvider.GUID = rotationTestK1
	cr.Status.AtProvider.RetiredKeys = []*v1alpha1.SCBResource{retired}
	store := newRotationStoredClient(cr)
	store.statusUpdateErr = errors.New("status write rejected")
	cf := &fake.MockServiceCredentialBinding{}
	active := rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationSucceeded)
	cf.On("Update", mock.Anything, rotationTestK1, mock.Anything).Return(active, nil).Twice()
	cf.On("Get", mock.Anything, rotationTestK1).Return(active, nil).Twice()
	cf.On("Get", mock.Anything, rotationTestK0).Return(rotationControllerBinding(rotationTestK0, v1alpha1.LastOperationSucceeded), nil).Once()
	cf.On("Delete", mock.Anything, rotationTestK0).Return("delete-job", nil).Once()
	cf.On("Get", mock.Anything, rotationTestK0).Return((*cfresource.ServiceCredentialBinding)(nil), cfresource.NewResourceNotFoundError()).Twice()

	_, err := rotationCleanupUpdate(t, store, cf, now)
	require.ErrorContains(t, err, "cannot persist retired-key cleanup progress")
	require.Len(t, store.get().Status.AtProvider.RetiredKeys, 1, "failed status persistence must leave API-server tracking intact")

	store.statusUpdateErr = nil
	_, err = rotationCleanupUpdate(t, store, cf, now)
	require.NoError(t, err, "a fresh reconciliation should confirm absence without resubmitting DELETE")
	require.Empty(t, store.get().Status.AtProvider.RetiredKeys)
	cf.AssertExpectations(t)
}

func assertRotationCleanupReconcile(t *testing.T, store *rotationStoredClient, cf *fake.MockServiceCredentialBinding, now time.Time) {
	t.Helper()
	_, err := rotationCleanupUpdate(t, store, cf, now)
	require.ErrorIs(t, err, servicecredentialbinding.ErrDeletionPending)
}

func rotationCleanupUpdate(t *testing.T, store *rotationStoredClient, cf *fake.MockServiceCredentialBinding, now time.Time) (managed.ExternalUpdate, error) {
	t.Helper()
	cr := store.get() // each invocation models a fresh cache/object after reconciliation or restart
	ext := rotationControllerExternal(store, cf)
	ext.nowFn = func() time.Time { return now }
	ext.keyRotator.(*servicecredentialbinding.SCBKeyRotator).Now = func() time.Time { return now }
	return ext.Update(context.Background(), cr)
}
