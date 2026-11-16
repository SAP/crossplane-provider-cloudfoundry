package servicecredentialbinding

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/fake"
	scb "github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/servicecredentialbinding"
)

type rotationFailingReader struct {
	client.Reader
	calls int
}

func (r *rotationFailingReader) Get(context.Context, client.ObjectKey, client.Object, ...client.GetOption) error {
	r.calls++
	return errors.New("secret reader unavailable")
}

func TestRotationDeletionDoesNotDependOnHandover(t *testing.T) {
	for _, bindingType := range []string{"key", "app"} {
		for _, operationState := range []string{
			v1alpha1.LastOperationSucceeded,
			v1alpha1.LastOperationInProgress,
			v1alpha1.LastOperationFailed,
		} {
			t.Run(bindingType+"/"+operationState, func(t *testing.T) {
				cr := rotationControllerCR(rotationTestK0, map[string]string{scb.ForceRotationKey: rotationTestK0})
				cr.Spec.ForProvider.Type = bindingType
				cr.Spec.WriteConnectionSecretToReference = &xpv2.SecretReference{Name: "rotation-credentials", Namespace: "rotation-test"}
				cr.Spec.ManagementPolicies = xpv2.ManagementPolicies{xpv2.ManagementActionAll}
				cr.Spec.DeletionPolicy = xpv2.DeletionDelete
				cr.Finalizers = []string{managed.FinalizerName}
				deleting := metav1.NewTime(time.Now())
				cr.SetDeletionTimestamp(&deleting)
				cr.Status.AtProvider.RetiredKeys = []*v1alpha1.SCBResource{{GUID: rotationTestK1}}

				cf := &fake.MockServiceCredentialBinding{}
				cf.On("Get", mock.Anything, rotationTestK0).Return(rotationControllerBinding(rotationTestK0, operationState), nil).Twice()
				direct := rotationRuntimeExternal(nil, cf)
				directReader := &rotationFailingReader{}
				direct.reader = directReader
				obs, err := direct.Observe(context.Background(), cr)
				require.NoError(t, err)
				require.True(t, obs.ResourceExists)
				require.Zero(t, directReader.calls)
				require.Len(t, cr.Status.AtProvider.RetiredKeys, 1)
				require.Nil(t, cr.Status.AtProvider.RetiredKeys[0].DeleteAfter)

				store, scheme := rotationRuntimeStore(t, cr)
				cf.On("Get", mock.Anything, rotationTestK1).Return(rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationSucceeded), nil).Once()
				cf.On("Delete", mock.Anything, rotationTestK1).Return("", nil).Once()
				cf.On("Get", mock.Anything, rotationTestK1).Return((*cfresource.ServiceCredentialBinding)(nil), cfresource.NewResourceNotFoundError()).Once()
				cf.On("Get", mock.Anything, rotationTestK0).Return(rotationControllerBinding(rotationTestK0, operationState), nil).Once()
				cf.On("Delete", mock.Anything, rotationTestK0).Return("", nil).Once()
				cf.On("Get", mock.Anything, rotationTestK0).Return((*cfresource.ServiceCredentialBinding)(nil), cfresource.NewResourceNotFoundError()).Once()
				external := rotationRuntimeExternal(store, cf)
				reader := &rotationFailingReader{}
				external.reader = reader
				reconciler := rotationRuntimeReconciler(t, store, scheme, external, true)

				_, err = reconciler.Reconcile(context.Background(), reconcile.Request{NamespacedName: client.ObjectKeyFromObject(cr)})
				require.NoError(t, err)
				require.Zero(t, reader.calls, "full deletion must not read the configured connection Secret")
				cf.AssertNotCalled(t, "GetDetails", mock.Anything, mock.Anything)
				cf.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
				cf.AssertExpectations(t)
			})
		}
	}
}

func TestRotationMissingActiveWaitsForRetiredDeletion(t *testing.T) {
	cr := rotationControllerCR(rotationTestK1, nil)
	cr.Spec.ManagementPolicies = xpv2.ManagementPolicies{xpv2.ManagementActionAll}
	cr.Spec.DeletionPolicy = xpv2.DeletionDelete
	cr.Finalizers = []string{managed.FinalizerName}
	deleting := metav1.NewTime(time.Now())
	cr.SetDeletionTimestamp(&deleting)
	cr.Status.AtProvider.RetiredKeys = []*v1alpha1.SCBResource{{GUID: rotationTestK0}}
	directCF := &fake.MockServiceCredentialBinding{}
	directCF.On("Get", mock.Anything, rotationTestK1).Return((*cfresource.ServiceCredentialBinding)(nil), cfresource.NewResourceNotFoundError()).Once()
	pending := rotationControllerBinding(rotationTestK0, v1alpha1.LastOperationInProgress)
	pending.LastOperation.Type = "delete"
	directCF.On("Get", mock.Anything, rotationTestK0).Return(pending, nil).Once()
	direct := rotationRuntimeExternal(nil, directCF)
	obs, err := direct.Observe(context.Background(), cr.DeepCopy())
	require.NoError(t, err, "pending retired cleanup is normal progress during deletion")
	require.True(t, obs.ResourceExists, "a tracked retiree means the aggregate external resource is not gone")
	require.True(t, obs.ResourceUpToDate)
	directCF.AssertExpectations(t)

	store, scheme := rotationRuntimeStore(t, cr)

	pending = rotationControllerBinding(rotationTestK0, v1alpha1.LastOperationInProgress)
	pending.LastOperation.Type = "delete"
	cf := &fake.MockServiceCredentialBinding{}
	cf.On("Get", mock.Anything, rotationTestK1).Return((*cfresource.ServiceCredentialBinding)(nil), cfresource.NewResourceNotFoundError()).Maybe()
	cf.On("Get", mock.Anything, rotationTestK0).Run(func(args mock.Arguments) {
		require.Equal(t, v1alpha1.LastOperationDelete, pending.LastOperation.Type)
		require.Equal(t, v1alpha1.LastOperationInProgress, pending.LastOperation.State)
	}).Return(pending, nil).Twice()
	cf.On("Get", mock.Anything, rotationTestK0).Return((*cfresource.ServiceCredentialBinding)(nil), cfresource.NewResourceNotFoundError()).Once()
	external := rotationRuntimeExternal(store, cf)
	recorder := &fakeRecorder{}
	reconciler := rotationRuntimeReconciler(t, store, scheme, external, true, managed.WithRecorder(recorder))
	request := reconcile.Request{NamespacedName: client.ObjectKeyFromObject(cr)}

	result, err := reconciler.Reconcile(context.Background(), request)
	require.NoError(t, err, "pending deletion is normal progress")
	require.True(t, runtimeRequestsRequeue(result), "runtime should retry pending external deletion")
	// The finalizer and K0 remain until a later observation confirms absence.
	remaining := &v1alpha1.ServiceCredentialBinding{}
	require.NoError(t, store.Get(context.Background(), client.ObjectKeyFromObject(cr), remaining))
	require.Contains(t, remaining.Finalizers, managed.FinalizerName)
	require.Len(t, remaining.Status.AtProvider.RetiredKeys, 1)
	require.Equal(t, rotationTestK0, remaining.Status.AtProvider.RetiredKeys[0].GUID)
	require.Equal(t, xpv2.ReasonReconcileSuccess, remaining.GetCondition(xpv2.TypeSynced).Reason)
	for _, recorded := range recorder.events {
		require.NotEqual(t, event.TypeWarning, recorded.Type, "normal pending deletion must not emit warning events")
	}
	cf.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)

	_, err = reconciler.Reconcile(context.Background(), request)
	require.NoError(t, err, "confirmed retired-binding absence should allow finalization")
	err = store.Get(context.Background(), client.ObjectKeyFromObject(cr), remaining)
	require.True(t, apierrors.IsNotFound(err), "the resource should finalize only after CF confirms the retired GUID is absent")
	cf.AssertExpectations(t)
}

func TestRotationRuntimePendingRetiredDeletionDoesNotAdvanceToActiveKey(t *testing.T) {
	cr := rotationControllerCR(rotationTestK1, nil)
	cr.Spec.ManagementPolicies = xpv2.ManagementPolicies{xpv2.ManagementActionAll}
	cr.Spec.DeletionPolicy = xpv2.DeletionDelete
	cr.Finalizers = []string{managed.FinalizerName}
	deleting := metav1.NewTime(time.Now())
	cr.SetDeletionTimestamp(&deleting)
	cr.Status.AtProvider.RetiredKeys = []*v1alpha1.SCBResource{{GUID: rotationTestK0}}
	store, scheme := rotationRuntimeStore(t, cr)
	cf := &fake.MockServiceCredentialBinding{}
	cf.On("Get", mock.Anything, rotationTestK1).Return(rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationSucceeded), nil).Once()
	pending := rotationControllerBinding(rotationTestK0, v1alpha1.LastOperationInProgress)
	pending.LastOperation.Type = v1alpha1.LastOperationDelete
	cf.On("Get", mock.Anything, rotationTestK0).Return(pending, nil).Once()
	recorder := &fakeRecorder{}
	external := rotationRuntimeExternal(store, cf)
	reconciler := rotationRuntimeReconciler(t, store, scheme, external, true, managed.WithRecorder(recorder))

	result, err := reconciler.Reconcile(context.Background(), reconcile.Request{NamespacedName: client.ObjectKeyFromObject(cr)})
	require.NoError(t, err)
	require.True(t, runtimeRequestsRequeue(result))
	remaining := &v1alpha1.ServiceCredentialBinding{}
	require.NoError(t, store.Get(context.Background(), client.ObjectKeyFromObject(cr), remaining))
	require.Contains(t, remaining.Finalizers, managed.FinalizerName)
	require.Len(t, remaining.Status.AtProvider.RetiredKeys, 1)
	require.Equal(t, xpv2.ReasonReconcileSuccess, remaining.GetCondition(xpv2.TypeSynced).Reason)
	for _, recorded := range recorder.events {
		require.NotEqual(t, event.TypeWarning, recorded.Type)
	}
	cf.AssertNotCalled(t, "Delete", mock.Anything, rotationTestK1)
	cf.AssertExpectations(t)
}

func TestRotationRuntimePendingActiveDeletionIsSuccessfulProgress(t *testing.T) {
	cr := rotationControllerCR(rotationTestK0, nil)
	cr.Spec.ManagementPolicies = xpv2.ManagementPolicies{xpv2.ManagementActionAll}
	cr.Spec.DeletionPolicy = xpv2.DeletionDelete
	cr.Finalizers = []string{managed.FinalizerName}
	deleting := metav1.NewTime(time.Now())
	cr.SetDeletionTimestamp(&deleting)
	store, scheme := rotationRuntimeStore(t, cr)
	cf := &fake.MockServiceCredentialBinding{}
	inProgress := rotationControllerBinding(rotationTestK0, v1alpha1.LastOperationInProgress)
	inProgress.LastOperation.Type = v1alpha1.LastOperationDelete
	cf.On("Get", mock.Anything, rotationTestK0).Return(inProgress, nil).Twice()
	recorder := &fakeRecorder{}
	external := rotationRuntimeExternal(store, cf)
	reconciler := rotationRuntimeReconciler(t, store, scheme, external, true, managed.WithRecorder(recorder))

	result, err := reconciler.Reconcile(context.Background(), reconcile.Request{NamespacedName: client.ObjectKeyFromObject(cr)})
	require.NoError(t, err)
	require.True(t, runtimeRequestsRequeue(result))
	remaining := &v1alpha1.ServiceCredentialBinding{}
	require.NoError(t, store.Get(context.Background(), client.ObjectKeyFromObject(cr), remaining))
	require.Contains(t, remaining.Finalizers, managed.FinalizerName)
	require.Equal(t, xpv2.ReasonReconcileSuccess, remaining.GetCondition(xpv2.TypeSynced).Reason)
	for _, recorded := range recorder.events {
		require.NotEqual(t, event.TypeWarning, recorded.Type, "expected active unbind progress must not emit a warning")
	}
	cf.AssertNotCalled(t, "Delete", mock.Anything, rotationTestK0)
	cf.AssertExpectations(t)
}

func TestRotationRuntimeDeleteFailureRemainsVisible(t *testing.T) {
	cr := rotationControllerCR(rotationTestK0, nil)
	cr.Spec.ManagementPolicies = xpv2.ManagementPolicies{xpv2.ManagementActionAll}
	cr.Spec.DeletionPolicy = xpv2.DeletionDelete
	cr.Finalizers = []string{managed.FinalizerName}
	deleting := metav1.NewTime(time.Now())
	cr.SetDeletionTimestamp(&deleting)
	store, scheme := rotationRuntimeStore(t, cr)
	cf := &fake.MockServiceCredentialBinding{}
	cf.On("Get", mock.Anything, rotationTestK0).Return(rotationControllerBinding(rotationTestK0, v1alpha1.LastOperationSucceeded), nil).Once()
	cf.On("Get", mock.Anything, rotationTestK0).Return((*cfresource.ServiceCredentialBinding)(nil), errors.New("Cloud Foundry lookup unavailable")).Once()
	recorder := &fakeRecorder{}
	external := rotationRuntimeExternal(store, cf)
	reconciler := rotationRuntimeReconciler(t, store, scheme, external, true, managed.WithRecorder(recorder))

	_, err := reconciler.Reconcile(context.Background(), reconcile.Request{NamespacedName: client.ObjectKeyFromObject(cr)})
	require.NoError(t, err, "runtime reports external errors through managed conditions/events")
	remaining := &v1alpha1.ServiceCredentialBinding{}
	require.NoError(t, store.Get(context.Background(), client.ObjectKeyFromObject(cr), remaining))
	require.Contains(t, remaining.Finalizers, managed.FinalizerName)
	require.Equal(t, xpv2.ReasonReconcileError, remaining.GetCondition(xpv2.TypeSynced).Reason)
	require.NotEmpty(t, recorder.events)
	require.Equal(t, event.TypeWarning, recorder.events[len(recorder.events)-1].Type)
	cf.AssertNotCalled(t, "Delete", mock.Anything, rotationTestK0)
	cf.AssertExpectations(t)
}

func runtimeRequestsRequeue(result reconcile.Result) bool {
	// The pinned runtime still uses Result.Requeue for its normal deletion poll,
	// although controller-runtime marks that field deprecated.
	return reflect.ValueOf(result).FieldByName("Requeue").Bool() || result.RequeueAfter > 0
}

func TestRotationDeletionDoesNotDeleteAfterBindingLookupFailure(t *testing.T) {
	cr := rotationControllerCR(rotationTestK0, nil)
	cr.Spec.ManagementPolicies = xpv2.ManagementPolicies{xpv2.ManagementActionAll}
	cr.Spec.DeletionPolicy = xpv2.DeletionDelete
	deleting := metav1.NewTime(time.Now())
	cr.SetDeletionTimestamp(&deleting)

	cf := &fake.MockServiceCredentialBinding{}
	cf.On("Get", mock.Anything, rotationTestK0).Return(rotationControllerBinding(rotationTestK0, v1alpha1.LastOperationSucceeded), errors.New("CF lookup unavailable")).Once()
	external := rotationRuntimeExternal(nil, cf)
	external.managementPoliciesEnabled = true

	_, err := external.Observe(context.Background(), cr)
	require.Error(t, err)
	cf.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
	cf.AssertExpectations(t)
}

var _ client.Reader = (*rotationFailingReader)(nil)
