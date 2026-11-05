package servicecredentialbinding

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/fake"
	scb "github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/servicecredentialbinding"
)

func TestRotationLegacyRetireeDeadlineStartsAtConfirmedHandover(t *testing.T) {
	ctx := context.Background()
	handover := time.Date(2026, 10, 28, 9, 30, 0, 0, time.UTC)
	now := handover
	cr := rotationControllerCR(rotationTestK1, nil)
	cr.Status.AtProvider.RetiredKeys = []*v1alpha1.SCBResource{{
		GUID:      rotationTestK0,
		CreatedAt: &metav1.Time{Time: handover.Add(-180 * 24 * time.Hour)},
	}}
	store := newRotationStoredClient(cr)
	cf := &fake.MockServiceCredentialBinding{}
	cf.On("Get", mock.Anything, rotationTestK1).Return(rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationSucceeded), nil).Twice()
	c := rotationControllerExternal(store, cf)
	c.nowFn = func() time.Time { return now }
	c.keyRotator.(*scb.SCBKeyRotator).Now = func() time.Time { return now }

	obs, err := c.Observe(ctx, cr)
	require.NoError(t, err)
	require.True(t, obs.ResourceExists)
	persisted := store.get()
	require.Len(t, persisted.Status.AtProvider.RetiredKeys, 1)
	deadline := persisted.Status.AtProvider.RetiredKeys[0].DeleteAfter
	require.NotNil(t, deadline)
	require.True(t, deadline.Time.Equal(handover.Add(48*time.Hour)),
		"legacy creation age must not consume any portion of the post-handover fallback")

	// A later reconciliation/reload must not slide the one-time deadline.
	now = handover.Add(24 * time.Hour)
	obs, err = c.Observe(ctx, store.get())
	require.NoError(t, err)
	require.True(t, obs.ResourceExists)
	require.Equal(t, deadline.Time, store.get().Status.AtProvider.RetiredKeys[0].DeleteAfter.Time)
	cf.AssertExpectations(t)
}

func TestRotationDoesNotInitializeFallbackBeforeReplacementHandover(t *testing.T) {
	t.Run("replacement still in progress", func(t *testing.T) {
		cr := rotationControllerCR(rotationTestK1, nil)
		cr.Status.AtProvider.RetiredKeys = []*v1alpha1.SCBResource{{GUID: rotationTestK0}}
		store := newRotationStoredClient(cr)
		cf := &fake.MockServiceCredentialBinding{}
		cf.On("Get", mock.Anything, rotationTestK1).Return(rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationInProgress), nil).Once()
		obs, err := rotationControllerExternal(store, cf).Observe(context.Background(), cr)
		require.NoError(t, err)
		require.True(t, obs.ResourceExists)
		require.Nil(t, store.get().Status.AtProvider.RetiredKeys[0].DeleteAfter)
		cf.AssertExpectations(t)
	})

	t.Run("retired identity is still active", func(t *testing.T) {
		cr := rotationControllerCR(rotationTestK0, nil)
		cr.Status.AtProvider.RetiredKeys = []*v1alpha1.SCBResource{{GUID: rotationTestK0}}
		store := newRotationStoredClient(cr)
		cf := &fake.MockServiceCredentialBinding{}
		cf.On("Get", mock.Anything, rotationTestK0).Return(rotationControllerBinding(rotationTestK0, v1alpha1.LastOperationSucceeded), nil).Once()
		obs, err := rotationControllerExternal(store, cf).Observe(context.Background(), cr)
		require.NoError(t, err)
		require.False(t, obs.ResourceExists, "accepted replacement must continue before assigning fallback retention")
		require.Nil(t, store.get().Status.AtProvider.RetiredKeys[0].DeleteAfter)
		cf.AssertExpectations(t)
	})
}
