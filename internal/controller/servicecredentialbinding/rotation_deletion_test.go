package servicecredentialbinding

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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
				cf.On("Delete", mock.Anything, rotationTestK1).Return("", nil).Once()
				cf.On("Delete", mock.Anything, rotationTestK0).Return("", nil).Once()
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
