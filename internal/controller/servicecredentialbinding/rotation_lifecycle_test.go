package servicecredentialbinding

import (
	"context"
	"errors"
	"testing"

	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/fake"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/servicecredentialbinding"
)

func rotationControllerBinding(guid string, state string) *cfresource.ServiceCredentialBinding {
	return &fake.NewServiceCredentialBinding("key").
		SetGUID(guid).
		SetName("service-key-suffix").
		SetServiceInstanceRef(serviceInstanceGUID).
		SetLastOperation(v1alpha1.LastOperationCreate, state).ServiceCredentialBinding
}

func rotationControllerCR(guid string, annotations map[string]string) *v1alpha1.ServiceCredentialBinding {
	cr := serviceCredentialBinding("key", withExternalName(guid), withServiceInstanceID(serviceInstanceGUID), withDefaultMetadata())
	cr.Spec.ForProvider.ServiceInstance = &serviceInstanceGUID
	cr.Annotations = annotations
	meta.SetExternalName(cr, guid)
	return cr
}

func rotationControllerExternal(kube *rotationStoredClient, cf *fake.MockServiceCredentialBinding) *external {
	cf.On("GetDetails", mock.Anything, mock.Anything).Return(&cfresource.ServiceCredentialBindingDetails{Credentials: map[string]any{}}, nil).Maybe()
	c := &external{
		kube:      kube,
		scbClient: cf,
		keyRotator: &servicecredentialbinding.SCBKeyRotator{
			SCBClient: cf,
		},
		maxCreateAttempts: DefaultMaxCreateAttempts,
	}
	c.observationStateHandler = c
	return c
}

func TestRotationCreateRetainsForceAnnotation(t *testing.T) {
	ctx := context.Background()
	cr := rotationControllerCR(rotationTestK0, map[string]string{
		servicecredentialbinding.ForceRotationKey: rotationTestK0,
	})
	store := newRotationStoredClient(cr)
	cf := &fake.MockServiceCredentialBinding{}
	cf.On("Create", mock.Anything, mock.Anything).Return("", rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationSucceeded), nil).Once()
	cf.On("Single", mock.Anything, mock.Anything).Return(rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationSucceeded), nil).Once()

	_, err := rotationControllerExternal(store, cf).Create(ctx, cr)
	require.NoError(t, err)
	require.Equal(t, rotationTestK0, cr.GetAnnotations()[servicecredentialbinding.ForceRotationKey])
	cf.AssertExpectations(t)
}

func TestRotationCriticalAnnotationConflict(t *testing.T) {
	ctx := context.Background()
	initial := rotationControllerCR(rotationTestK0, map[string]string{
		servicecredentialbinding.ForceRotationKey: rotationTestK0,
	})
	store := newRotationStoredClient(initial)
	cf := &fake.MockServiceCredentialBinding{}
	cf.On("Create", mock.Anything, mock.Anything).Return("", rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationSucceeded), nil).Once()
	cf.On("Single", mock.Anything, mock.Anything).Return(rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationSucceeded), nil).Once()
	cf.On("Get", mock.Anything, rotationTestK0).Return(rotationControllerBinding(rotationTestK0, v1alpha1.LastOperationSucceeded), nil).Once()

	c := rotationControllerExternal(store, cf)
	obs, err := c.Observe(ctx, initial)
	require.NoError(t, err)
	require.False(t, obs.ResourceExists)
	require.Len(t, store.get().Status.AtProvider.RetiredKeys, 1)
	require.Equal(t, rotationTestK0, store.get().Status.AtProvider.RetiredKeys[0].GUID)

	_, err = c.Create(ctx, initial)
	require.NoError(t, err)
	// Create retains the operational target; it becomes inert once the active
	// Cloud Foundry generation changes.
	require.Equal(t, rotationTestK0, initial.GetAnnotations()[servicecredentialbinding.ForceRotationKey])
	require.Contains(t, store.get().GetAnnotations(), servicecredentialbinding.ForceRotationKey)
	require.Equal(t, rotationTestK1, meta.GetExternalName(initial))

	store.conflictNext = true
	updater := managed.NewRetryingCriticalAnnotationUpdater(store.MockClient)
	require.NoError(t, updater.UpdateCriticalAnnotations(ctx, initial))

	persisted := store.get()
	require.Equal(t, rotationTestK1, meta.GetExternalName(persisted))
	require.Equal(t, rotationTestK0, persisted.GetAnnotations()[servicecredentialbinding.ForceRotationKey])
	cf.AssertExpectations(t)

	// Reconciliation after conflict/reload must treat the retained K0 target
	// as stale when Cloud Foundry reports K1 as the active generation.
	reloaded := store.get()
	cf.On("Get", mock.Anything, rotationTestK1).Return(rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationSucceeded), nil).Once()
	obs, err = rotationControllerExternal(store, cf).Observe(ctx, reloaded)
	require.NoError(t, err)
	require.True(t, obs.ResourceExists, "stale K0 annotation must not request retirement of K1")
	require.Len(t, store.get().Status.AtProvider.RetiredKeys, 1)
	require.Equal(t, rotationTestK0, store.get().Status.AtProvider.RetiredKeys[0].GUID)
	require.NotNil(t, store.get().Status.AtProvider.RetiredKeys[0].DeleteAfter)
	cf.AssertExpectations(t)
}

func TestRotationCriticalAnnotationConflictExhaustion(t *testing.T) {
	cr := rotationControllerCR(rotationTestK0, map[string]string{
		servicecredentialbinding.ForceRotationKey: rotationTestK0,
	})
	cr.Status.AtProvider.RetiredKeys = []*v1alpha1.SCBResource{{GUID: rotationTestK0}}
	store := newRotationStoredClient(cr)
	store.conflictsRemaining = 10
	local := store.get()
	meta.SetExternalName(local, rotationTestK1)
	delete(local.Annotations, servicecredentialbinding.ForceRotationKey)

	err := managed.NewRetryingCriticalAnnotationUpdater(store.MockClient).UpdateCriticalAnnotations(context.Background(), local)
	require.Error(t, err, "retry exhaustion must not imply the replacement identity was persisted")
	persisted := store.get()
	require.Equal(t, rotationTestK0, meta.GetExternalName(persisted))
	require.Equal(t, rotationTestK0, persisted.GetAnnotations()[servicecredentialbinding.ForceRotationKey])
	require.Len(t, persisted.Status.AtProvider.RetiredKeys, 1)
}

func TestRotationPendingReplacementRecovery(t *testing.T) {
	ctx := context.Background()
	cr := rotationControllerCR(rotationTestK0, map[string]string{
		servicecredentialbinding.ForceRotationKey: rotationTestK0,
	})
	store := newRotationStoredClient(cr)
	cf := &fake.MockServiceCredentialBinding{}
	cf.On("Create", mock.Anything, mock.Anything).Return("", fake.ServiceCredentialBindingNil, errCFClientError).Once()
	cf.On("Get", mock.Anything, rotationTestK0).Return(rotationControllerBinding(rotationTestK0, v1alpha1.LastOperationSucceeded), nil).Twice()
	cf.On("GetDetails", mock.Anything, rotationTestK0).Return(&cfresource.ServiceCredentialBindingDetails{Credentials: map[string]any{}}, nil).Once()

	// Persist the old identity first, then simulate process reload and a definite
	// pre-creation failure. Removing the now-accepted annotation must not cancel it.
	obs, err := rotationControllerExternal(store, cf).Observe(ctx, cr)
	require.NoError(t, err)
	require.False(t, obs.ResourceExists)
	require.Len(t, store.get().Status.AtProvider.RetiredKeys, 1)
	_, createErr := rotationControllerExternal(store, cf).Create(ctx, store.get())
	require.Error(t, createErr)
	delete(store.stored.Annotations, servicecredentialbinding.ForceRotationKey)
	obs, err = rotationControllerExternal(store, cf).Observe(ctx, store.get())
	require.NoError(t, err)
	require.False(t, obs.ResourceExists, "persisted retirement must resume replacement after the original Create failed")
	require.Len(t, store.get().Status.AtProvider.RetiredKeys, 1)
	require.Equal(t, rotationTestK0, store.get().Status.AtProvider.RetiredKeys[0].GUID)
	cf.AssertExpectations(t)
}

func TestRotationFallbackDeadlinePersistenceFailureDefersNewRequest(t *testing.T) {
	cr := rotationControllerCR(rotationTestK1, map[string]string{servicecredentialbinding.ForceRotationKey: rotationTestK1})
	cr.Status.AtProvider.RetiredKeys = []*v1alpha1.SCBResource{{GUID: rotationTestK0}}
	store := newRotationStoredClient(cr)
	store.statusUpdateErr = errors.New("deadline status write rejected")
	cf := &fake.MockServiceCredentialBinding{}
	cf.On("Get", mock.Anything, rotationTestK1).Return(rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationSucceeded), nil).Once()

	obs, err := rotationControllerExternal(store, cf).Observe(context.Background(), cr)
	require.Error(t, err)
	require.Empty(t, obs)
	require.Len(t, store.get().Status.AtProvider.RetiredKeys, 1, "failed deadline persistence must not retire the active generation")
	require.Equal(t, rotationTestK0, store.get().Status.AtProvider.RetiredKeys[0].GUID)
	require.Nil(t, store.get().Status.AtProvider.RetiredKeys[0].DeleteAfter)
	require.Equal(t, []string{"StatusUpdate"}, store.calls)
	cf.AssertExpectations(t)
}

func TestRotationRetirementPersistenceFailure(t *testing.T) {
	ctx := context.Background()
	cr := rotationControllerCR(rotationTestK0, map[string]string{
		servicecredentialbinding.ForceRotationKey: rotationTestK0,
	})
	store := newRotationStoredClient(cr)
	store.statusUpdateErr = errors.New("status write rejected")
	cf := &fake.MockServiceCredentialBinding{}
	cf.On("Get", mock.Anything, rotationTestK0).Return(rotationControllerBinding(rotationTestK0, v1alpha1.LastOperationSucceeded), nil).Once()

	obs, err := rotationControllerExternal(store, cf).Observe(ctx, cr)
	require.Error(t, err)
	require.Empty(t, obs)
	require.Empty(t, store.get().Status.AtProvider.RetiredKeys, "failed status persistence must not record retirement")
	require.Equal(t, []string{"StatusUpdate"}, store.calls)
	cf.AssertExpectations(t)
}

func TestRotationKnownIdentityOnCreateError(t *testing.T) {
	ctx := context.Background()
	cr := rotationControllerCR(rotationTestK0, map[string]string{
		servicecredentialbinding.ForceRotationKey: rotationTestK0,
	})
	store := newRotationStoredClient(cr)
	cf := &fake.MockServiceCredentialBinding{}
	cf.On("Create", mock.Anything, mock.Anything).Return("job-k1", rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationInProgress), nil).Once()
	cf.On("PollComplete", mock.Anything, "job-k1", mock.Anything).Return(errCFClientError).Once()
	cf.On("Single", mock.Anything, mock.Anything).Return(rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationInProgress), nil).Once()

	c := rotationControllerExternal(store, cf)
	_, err := c.Create(ctx, cr)
	require.Error(t, err)
	require.Equal(t, rotationTestK1, meta.GetExternalName(cr), "known replacement identity must be captured despite the job error")
	require.NoError(t, managed.NewRetryingCriticalAnnotationUpdater(store.MockClient).UpdateCriticalAnnotations(ctx, cr))
	require.Equal(t, rotationTestK1, meta.GetExternalName(store.get()))

	cf.On("Get", mock.Anything, rotationTestK1).Return(rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationInProgress), nil).Once()
	obs, observeErr := rotationControllerExternal(store, cf).Observe(ctx, store.get())
	require.NoError(t, observeErr)
	require.True(t, obs.ResourceExists, "known in-progress K1 must be observed, not retired by stale K0")
	cf.AssertExpectations(t)
}
