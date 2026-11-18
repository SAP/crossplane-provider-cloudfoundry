package servicecredentialbinding

import (
	"context"
	"errors"
	"testing"
	"time"

	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/fake"
	scb "github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/servicecredentialbinding"
)

func TestRotationObserveReportsNextRotation(t *testing.T) {
	ctx := context.Background()
	created := time.Now().UTC().Truncate(time.Second)
	cr := rotationControllerCR(rotationTestK1, nil)
	cr.Spec.ForProvider.Rotation = &v1alpha1.RotationParameters{Frequency: &metav1.Duration{Duration: time.Hour}}
	binding := rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationSucceeded)
	binding.CreatedAt = created
	cf := &fake.MockServiceCredentialBinding{}
	cf.On("Get", mock.Anything, rotationTestK1).Return(binding, nil).Twice()
	c := rotationControllerExternal(newRotationStoredClient(cr), cf)

	_, err := c.Observe(ctx, cr)
	require.NoError(t, err)
	require.NotNil(t, cr.Status.AtProvider.NextRotationAt)
	require.True(t, cr.Status.AtProvider.NextRotationAt.Equal(&metav1.Time{Time: created.Add(time.Hour)}))

	cr.Spec.ForProvider.Rotation = nil
	_, err = c.Observe(ctx, cr)
	require.NoError(t, err)
	require.Nil(t, cr.Status.AtProvider.NextRotationAt, "removing the schedule must clear the reported due time")
	cf.AssertExpectations(t)
}

func TestRotationRecoveryPreservesAcceptedTrigger(t *testing.T) {
	for _, trigger := range []string{v1alpha1.RotationTriggerForced, v1alpha1.RotationTriggerScheduled} {
		t.Run(trigger, func(t *testing.T) {
			ctx := context.Background()
			now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
			cr := rotationControllerCR(rotationTestK0, nil)
			if trigger == v1alpha1.RotationTriggerForced {
				cr.Annotations[scb.ForceRotationKey] = rotationTestK0
			} else {
				cr.Spec.ForProvider.Rotation = &v1alpha1.RotationParameters{Frequency: &metav1.Duration{Duration: time.Hour}}
			}
			store := newRotationStoredClient(cr)
			old := rotationControllerBinding(rotationTestK0, v1alpha1.LastOperationSucceeded)
			old.CreatedAt = now.Add(-2 * time.Hour)
			replacement := rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationSucceeded)
			replacement.CreatedAt = now
			cf := &fake.MockServiceCredentialBinding{}
			cf.On("Get", mock.Anything, rotationTestK0).Return(old, nil).Twice()
			cf.On("Create", mock.Anything, mock.Anything).Return("", fake.ServiceCredentialBindingNil, errCFClientError).Once()
			cf.On("Create", mock.Anything, mock.Anything).Return("", replacement, nil).Once()
			cf.On("Single", mock.Anything, mock.Anything).Return(replacement, nil).Once()
			cf.On("Get", mock.Anything, rotationTestK1).Return(replacement, nil).Twice()
			c := rotationControllerExternal(store, cf)
			c.nowFn = func() time.Time { return now }
			c.keyRotator.(*scb.SCBKeyRotator).Now = func() time.Time { return now }
			recorder := &fakeRecorder{}
			c.recorder = recorder

			obs, err := c.Observe(ctx, store.get())
			require.NoError(t, err)
			require.False(t, obs.ResourceExists)
			_, err = c.Create(ctx, store.get())
			require.ErrorIs(t, err, errCFClientError)

			// Change the annotation and schedule; the event must still report
			// the trigger recorded at retirement.
			changed := store.get()
			if trigger == v1alpha1.RotationTriggerForced {
				delete(changed.Annotations, scb.ForceRotationKey)
			} else {
				changed.Annotations[scb.ForceRotationKey] = rotationTestK0
				changed.Spec.ForProvider.Rotation = nil
			}
			require.NoError(t, store.Update(ctx, changed))
			obs, err = c.Observe(ctx, store.get())
			require.NoError(t, err)
			require.False(t, obs.ResourceExists, "accepted replacement must resume after intent changes")
			require.Empty(t, eventsWithReason(recorder, reasonKeyRotated), "observing the retiring key is not completion")

			retry := store.get()
			_, err = c.Create(ctx, retry)
			require.NoError(t, err)
			require.NoError(t, managed.NewRetryingCriticalAnnotationUpdater(store.MockClient).UpdateCriticalAnnotations(ctx, retry))
			for range 2 {
				obs, err = c.Observe(ctx, store.get())
				require.NoError(t, err)
				require.True(t, obs.ResourceExists)
			}
			rotated := eventsWithReason(recorder, reasonKeyRotated)
			require.Len(t, rotated, 1, "reloaded observations must not announce the same replacement twice")
			require.Equal(t, event.TypeNormal, rotated[0].Type)
			require.Contains(t, rotated[0].Message, rotationTestK0)
			require.Contains(t, rotated[0].Message, rotationTestK1)
			require.Contains(t, rotated[0].Message, "("+trigger+")")
			cf.AssertExpectations(t)
		})
	}
}

func TestRotationRecoveredCreationReportsCompletion(t *testing.T) {
	for _, state := range []string{v1alpha1.LastOperationInProgress, v1alpha1.LastOperationFailed, v1alpha1.LastOperationSucceeded} {
		t.Run(state, func(t *testing.T) {
			ctx := context.Background()
			cr := rotationControllerCR(rotationTestK0, map[string]string{scb.ForceRotationKey: rotationTestK0})
			store := newRotationStoredClient(cr)
			cf := &fake.MockServiceCredentialBinding{}
			cf.On("Get", mock.Anything, rotationTestK0).Return(rotationControllerBinding(rotationTestK0, v1alpha1.LastOperationSucceeded), nil).Once()
			provisioning := rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationInProgress)
			cf.On("Create", mock.Anything, mock.Anything).Return("replacement-job", provisioning, nil).Once()
			cf.On("PollComplete", mock.Anything, "replacement-job", mock.Anything).Return(errCFClientError).Once()
			cf.On("Single", mock.Anything, mock.Anything).Return(provisioning, nil).Once()
			cf.On("Get", mock.Anything, rotationTestK1).Return(rotationControllerBinding(rotationTestK1, state), nil).Twice()
			c := rotationControllerExternal(store, cf)
			recorder := &fakeRecorder{}
			c.recorder = recorder

			obs, err := c.Observe(ctx, store.get())
			require.NoError(t, err)
			require.False(t, obs.ResourceExists)
			attempt := store.get()
			_, err = c.Create(ctx, attempt)
			require.ErrorIs(t, err, errCFClientError)
			require.Equal(t, rotationTestK1, meta.GetExternalName(attempt))
			require.NoError(t, managed.NewRetryingCriticalAnnotationUpdater(store.MockClient).UpdateCriticalAnnotations(ctx, attempt))
			require.Empty(t, eventsWithReason(recorder, reasonKeyRotated), "a polling error is not proof of success")

			for range 2 {
				obs, err = c.Observe(ctx, store.get())
				require.NoError(t, err)
				require.True(t, obs.ResourceExists)
			}
			rotated := eventsWithReason(recorder, reasonKeyRotated)
			if state != v1alpha1.LastOperationSucceeded {
				require.Empty(t, rotated, "an unavailable replacement must not be announced as rotated")
			} else {
				require.Len(t, rotated, 1, "successful recovery must be announced once")
				require.Contains(t, rotated[0].Message, rotationTestK0)
				require.Contains(t, rotated[0].Message, rotationTestK1)
				require.Contains(t, rotated[0].Message, "(forced)")
			}
			cf.AssertExpectations(t)
		})
	}
}

func TestRotationCompletionRetriesAfterStatusWriteFailure(t *testing.T) {
	ctx := context.Background()
	cr := rotationControllerCR(rotationTestK0, map[string]string{scb.ForceRotationKey: rotationTestK0})
	store := newRotationStoredClient(cr)
	cf := &fake.MockServiceCredentialBinding{}
	cf.On("Get", mock.Anything, rotationTestK0).Return(rotationControllerBinding(rotationTestK0, v1alpha1.LastOperationSucceeded), nil).Once()
	replacement := rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationSucceeded)
	cf.On("Create", mock.Anything, mock.Anything).Return("", replacement, nil).Once()
	cf.On("Single", mock.Anything, mock.Anything).Return(replacement, nil).Once()
	cf.On("Get", mock.Anything, rotationTestK1).Return(replacement, nil).Times(3)
	c := rotationControllerExternal(store, cf)
	recorder := &fakeRecorder{}
	c.recorder = recorder

	_, err := c.Observe(ctx, store.get())
	require.NoError(t, err)
	attempt := store.get()
	_, err = c.Create(ctx, attempt)
	require.NoError(t, err)
	require.NoError(t, managed.NewRetryingCriticalAnnotationUpdater(store.MockClient).UpdateCriticalAnnotations(ctx, attempt))

	store.statusUpdateErr = errors.New("status write rejected")
	observed := store.get()
	_, err = c.Observe(ctx, observed)
	require.Error(t, err)
	require.Empty(t, eventsWithReason(recorder, reasonKeyRotated), "completion must be persisted before its event")

	// Simulate the runtime writing the in-memory object back; that must not
	// mark the rotation complete.
	store.statusUpdateErr = nil
	require.NoError(t, store.Status().Update(ctx, observed))
	for range 2 {
		_, err = c.Observe(ctx, store.get())
		require.NoError(t, err)
	}
	require.Len(t, eventsWithReason(recorder, reasonKeyRotated), 1, "status recovery must not lose or duplicate completion")
	cf.AssertExpectations(t)
}

func TestRotationObservationDoesNotReplayLegacyRetirements(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "first key", true: "legacy retired key"}[legacy], func(t *testing.T) {
			cr := rotationControllerCR(rotationTestK1, nil)
			if legacy {
				cr.Status.AtProvider.RetiredKeys = []*v1alpha1.SCBResource{{GUID: rotationTestK0}}
			}
			cf := &fake.MockServiceCredentialBinding{}
			cf.On("Get", mock.Anything, rotationTestK1).Return(rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationSucceeded), nil).Once()
			c := rotationControllerExternal(newRotationStoredClient(cr), cf)
			recorder := &fakeRecorder{}
			c.recorder = recorder

			_, err := c.Observe(context.Background(), cr)
			require.NoError(t, err)
			require.Empty(t, eventsWithReason(recorder, reasonKeyRotated), "ordinary observation must not fabricate a rotation")
			cf.AssertExpectations(t)
		})
	}
}

func TestRotationObserveReportsSecretRepublish(t *testing.T) {
	for name, tc := range map[string]struct {
		withSecret bool
		mutate     func(*corev1.Secret)
		wantEvent  bool
	}{
		"stale controlled Secret": {withSecret: true, wantEvent: true},
		"first publish":           {},
		"current Secret": {
			withSecret: true,
			mutate:     func(s *corev1.Secret) { s.Data = map[string][]byte{"username": []byte("new")} },
		},
		"Secret controlled by another owner": {
			withSecret: true,
			mutate:     func(s *corev1.Secret) { s.OwnerReferences = nil },
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			cr := rotationHandoverCR(rotationTestK1, "__absent__", "key")
			kube := rotationHandoverKube(t, cr, tc.withSecret)
			if tc.mutate != nil {
				secret := &corev1.Secret{}
				require.NoError(t, kube.Get(ctx, types.NamespacedName{Namespace: "rotation-test", Name: "rotation-credentials"}, secret))
				tc.mutate(secret)
				require.NoError(t, kube.Update(ctx, secret))
			}
			cf := &fake.MockServiceCredentialBinding{}
			cf.On("Get", mock.Anything, rotationTestK1).Return(rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationSucceeded), nil)
			cf.On("GetDetails", mock.Anything, rotationTestK1).Return(&cfresource.ServiceCredentialBindingDetails{Credentials: map[string]any{"username": "new"}}, nil)
			c := rotationRuntimeExternal(kube, cf)
			recorder := &fakeRecorder{}
			c.recorder = recorder

			obs, err := c.Observe(ctx, cr)
			require.NoError(t, err)
			require.Equal(t, []byte("new"), obs.ConnectionDetails["username"])
			republished := eventsWithReason(recorder, reasonConnectionSecretRepublished)
			if !tc.wantEvent {
				require.Empty(t, republished)
				return
			}
			require.Len(t, republished, 1)
			require.Equal(t, event.TypeNormal, republished[0].Type)
			require.Equal(t, "Connection Secret rotation-test/rotation-credentials lacks the credentials of key "+rotationTestK1+"; republishing it", republished[0].Message)
		})
	}
}

func TestRotationCleanupEvents(t *testing.T) {
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	cr := rotationCleanupEventCR(now, rotationTestK0, rotationTestK2)
	store := newRotationStoredClient(cr)
	cf := rotationCleanupEventCF()
	expectRetiredKeyConfirmedDeleted(cf, rotationTestK0)
	cf.On("Get", mock.Anything, rotationTestK2).Return(rotationControllerBinding(rotationTestK2, v1alpha1.LastOperationSucceeded), nil).Once()
	cf.On("Delete", mock.Anything, rotationTestK2).Return("", errCFClientError).Once()
	recorder := &fakeRecorder{}

	_, err := rotationObservedCleanup(store, cf, recorder, now)
	require.ErrorIs(t, err, errCFClientError)
	require.Equal(t, []string{rotationTestK2}, retiredGUIDs(store.get()), "the confirmed deletion is persisted despite the other failure")

	deleted := eventsWithReason(recorder, reasonRetiredKeyDeleted)
	require.Len(t, deleted, 1)
	require.Equal(t, event.TypeNormal, deleted[0].Type)
	require.Contains(t, deleted[0].Message, rotationTestK0)
	failed := eventsWithReason(recorder, reasonCannotDeleteRetiredKey)
	require.Len(t, failed, 1)
	require.Equal(t, event.TypeWarning, failed[0].Type)
	require.Contains(t, failed[0].Message, rotationTestK2)
	require.Contains(t, failed[0].Message, errCFClientError.Error())
	cf.AssertExpectations(t)
}

func TestRotationCleanupDeletedEventFollowsPersistedStatus(t *testing.T) {
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	store := newRotationStoredClient(rotationCleanupEventCR(now, rotationTestK0))
	store.statusUpdateErr = errors.New("status write rejected")
	cf := rotationCleanupEventCF()
	expectRetiredKeyConfirmedDeleted(cf, rotationTestK0)
	cf.On("Get", mock.Anything, rotationTestK0).Return((*cfresource.ServiceCredentialBinding)(nil), cfresource.NewResourceNotFoundError()).Once()
	recorder := &fakeRecorder{}

	reconciled, err := rotationObservedCleanup(store, cf, recorder, now)
	require.Error(t, err)
	require.Empty(t, eventsWithReason(recorder, reasonRetiredKeyDeleted), "the key is still tracked until status is persisted")
	require.Equal(t, []string{rotationTestK0}, retiredGUIDs(reconciled), "the runtime's error status write must not drop the key without an event")

	store.statusUpdateErr = nil
	_, err = rotationObservedCleanup(store, cf, recorder, now)
	require.NoError(t, err)
	require.Empty(t, retiredGUIDs(store.get()))
	require.Len(t, eventsWithReason(recorder, reasonRetiredKeyDeleted), 1)
	cf.AssertExpectations(t)
}

func TestRotationCleanupPendingDeletionIsQuiet(t *testing.T) {
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	store := newRotationStoredClient(rotationCleanupEventCR(now, rotationTestK0))
	cf := rotationCleanupEventCF()
	inProgress := rotationControllerBinding(rotationTestK0, v1alpha1.LastOperationInProgress)
	inProgress.LastOperation.Type = v1alpha1.LastOperationDelete
	cf.On("Get", mock.Anything, rotationTestK0).Return(inProgress, nil).Once()
	recorder := &fakeRecorder{}

	_, err := rotationObservedCleanup(store, cf, recorder, now)
	require.NoError(t, err)
	require.Equal(t, []string{rotationTestK0}, retiredGUIDs(store.get()))
	require.Empty(t, recorder.events, "an unbind in progress is neither a deletion nor a failure")
	cf.AssertExpectations(t)
}

func TestRotationCleanupFailedUnbindWarns(t *testing.T) {
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	store := newRotationStoredClient(rotationCleanupEventCR(now, rotationTestK0))
	cf := rotationCleanupEventCF()
	failedUnbind := rotationControllerBinding(rotationTestK0, v1alpha1.LastOperationFailed)
	failedUnbind.LastOperation.Type = v1alpha1.LastOperationDelete
	failedUnbind.LastOperation.Description = "broker rejected unbind"
	cf.On("Get", mock.Anything, rotationTestK0).Return(failedUnbind, nil).Once()
	cf.On("Delete", mock.Anything, rotationTestK0).Return("retry-job", nil).Once()
	cf.On("Get", mock.Anything, rotationTestK0).Return(rotationControllerBinding(rotationTestK0, v1alpha1.LastOperationInProgress), nil).Once()
	recorder := &fakeRecorder{}

	_, err := rotationObservedCleanup(store, cf, recorder, now)
	require.ErrorIs(t, err, scb.ErrUnbindFailed)
	failed := eventsWithReason(recorder, reasonCannotDeleteRetiredKey)
	require.Len(t, failed, 1)
	require.Contains(t, failed[0].Message, "broker rejected unbind")
	cf.AssertExpectations(t)
}

func rotationCleanupEventCR(now time.Time, retired ...string) *v1alpha1.ServiceCredentialBinding {
	cr := serviceCredentialBinding("key", withServiceInstanceID(serviceInstanceGUID), withExternalName(rotationTestK1))
	cr.Status.AtProvider.GUID = rotationTestK1
	for _, guid := range retired {
		cr.Status.AtProvider.RetiredKeys = append(cr.Status.AtProvider.RetiredKeys, &v1alpha1.SCBResource{GUID: guid, DeleteAfter: &metav1.Time{Time: now.Add(-time.Second)}})
	}
	return cr
}

func rotationCleanupEventCF() *fake.MockServiceCredentialBinding {
	cf := &fake.MockServiceCredentialBinding{}
	active := rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationSucceeded)
	cf.On("Update", mock.Anything, rotationTestK1, mock.Anything).Return(active, nil)
	cf.On("Get", mock.Anything, rotationTestK1).Return(active, nil)
	return cf
}

func expectRetiredKeyConfirmedDeleted(cf *fake.MockServiceCredentialBinding, guid string) {
	cf.On("Get", mock.Anything, guid).Return(rotationControllerBinding(guid, v1alpha1.LastOperationSucceeded), nil).Once()
	cf.On("Delete", mock.Anything, guid).Return("", nil).Once()
	cf.On("Get", mock.Anything, guid).Return((*cfresource.ServiceCredentialBinding)(nil), cfresource.NewResourceNotFoundError()).Once()
}

// rotationObservedCleanup runs routine cleanup on a fresh copy of the stored object
// and returns that copy, which the runtime would write back as status.
func rotationObservedCleanup(store *rotationStoredClient, cf *fake.MockServiceCredentialBinding, recorder *fakeRecorder, now time.Time) (*v1alpha1.ServiceCredentialBinding, error) {
	cr := store.get()
	ext := rotationCleanupExternal(store, cf, now)
	ext.recorder = recorder
	_, err := ext.Update(context.Background(), cr)
	return cr, err
}

func retiredGUIDs(cr *v1alpha1.ServiceCredentialBinding) []string {
	guids := make([]string, 0, len(cr.Status.AtProvider.RetiredKeys))
	for _, key := range cr.Status.AtProvider.RetiredKeys {
		guids = append(guids, key.GUID)
	}
	return guids
}

func eventsWithReason(recorder *fakeRecorder, reason event.Reason) []event.Event {
	var matched []event.Event
	for _, e := range recorder.events {
		if e.Reason == reason {
			matched = append(matched, e)
		}
	}
	return matched
}
