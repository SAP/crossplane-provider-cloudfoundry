package servicecredentialbinding

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/fake"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/servicecredentialbinding"
)

func rotationHandoverCR(active, target string, bindingType string) *v1alpha1.ServiceCredentialBinding {
	annotations := map[string]string{}
	if target != "__absent__" {
		annotations[servicecredentialbinding.ForceRotationKey] = target
	}
	cr := rotationControllerCR(active, annotations)
	cr.Spec.ForProvider.Type = bindingType
	cr.UID = types.UID("rotation-test-owner")
	cr.Spec.WriteConnectionSecretToReference = &xpv2.SecretReference{Name: "rotation-credentials", Namespace: "rotation-test"}
	cr.Status.AtProvider.RetiredKeys = []*v1alpha1.SCBResource{{GUID: rotationTestK0}}
	return cr
}

func TestRotationHandoverDeferral(t *testing.T) {
	states := []struct {
		name  string
		state string
	}{
		{name: "initial", state: v1alpha1.LastOperationInitial},
		{name: "in progress", state: v1alpha1.LastOperationInProgress},
		{name: "failed", state: v1alpha1.LastOperationFailed},
		{name: "succeeded but secret not updated", state: v1alpha1.LastOperationSucceeded},
	}

	for _, tc := range states {
		t.Run(tc.name, func(t *testing.T) {
			cr := rotationHandoverCR(rotationTestK1, rotationTestK1, "key")
			cf := &fake.MockServiceCredentialBinding{}
			cf.On("Get", mock.Anything, rotationTestK1).Return(rotationControllerBinding(rotationTestK1, tc.state), nil).Once()
			cf.On("GetDetails", mock.Anything, rotationTestK1).Return(&cfresource.ServiceCredentialBindingDetails{Credentials: map[string]any{"username": "new"}}, nil).Maybe()
			kube := rotationHandoverKube(t, cr, true)
			obs, err := rotationRuntimeExternal(kube, cf).Observe(context.Background(), cr)
			assert.NoError(t, err)
			assert.True(t, obs.ResourceExists, "a replacement cannot be retired before handover")
			persisted := &v1alpha1.ServiceCredentialBinding{}
			if assert.NoError(t, kube.Get(context.Background(), types.NamespacedName{Name: cr.Name}, persisted)) {
				assert.Len(t, persisted.Status.AtProvider.RetiredKeys, 1)
				if len(persisted.Status.AtProvider.RetiredKeys) > 0 {
					assert.Equal(t, rotationTestK0, persisted.Status.AtProvider.RetiredKeys[0].GUID)
				}
			}
			cf.AssertExpectations(t)
		})
	}
}

func TestRotationHandoverSecretMismatchAndReadErrors(t *testing.T) {
	for _, tc := range []struct {
		name      string
		hasSecret bool
	}{
		{name: "missing secret"},
		{name: "secret has old credentials", hasSecret: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cr := rotationHandoverCR(rotationTestK1, rotationTestK1, "key")
			cf := &fake.MockServiceCredentialBinding{}
			cf.On("Get", mock.Anything, rotationTestK1).Return(rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationSucceeded), nil).Once()
			cf.On("GetDetails", mock.Anything, rotationTestK1).Return(&cfresource.ServiceCredentialBindingDetails{Credentials: map[string]any{"username": "new"}}, nil).Maybe()
			kube := rotationHandoverKube(t, cr, tc.hasSecret)
			obs, err := rotationRuntimeExternal(kube, cf).Observe(context.Background(), cr)
			assert.NoError(t, err)
			assert.True(t, obs.ResourceExists)
			persisted := &v1alpha1.ServiceCredentialBinding{}
			if assert.NoError(t, kube.Get(context.Background(), types.NamespacedName{Name: cr.Name}, persisted)) {
				assert.Len(t, persisted.Status.AtProvider.RetiredKeys, 1)
			}
			cf.AssertExpectations(t)
		})
	}
}

func TestRotationDeferredRequestRemovedBeforeHandover(t *testing.T) {
	cr := rotationHandoverCR(rotationTestK1, "__absent__", "key")
	cf := &fake.MockServiceCredentialBinding{}
	cf.On("Get", mock.Anything, rotationTestK1).Return(rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationInProgress), nil).Once()
	kube := rotationHandoverKube(t, cr, true)
	obs, err := rotationRuntimeExternal(kube, cf).Observe(context.Background(), cr)
	require.NoError(t, err)
	require.True(t, obs.ResourceExists)
	persisted := &v1alpha1.ServiceCredentialBinding{}
	require.NoError(t, kube.Get(context.Background(), types.NamespacedName{Name: cr.Name}, persisted))
	require.Len(t, persisted.Status.AtProvider.RetiredKeys, 1)
	require.Equal(t, rotationTestK0, persisted.Status.AtProvider.RetiredKeys[0].GUID)
	cf.AssertExpectations(t)
}

func TestRotationHandoverNoSecretPositiveControl(t *testing.T) {
	cr := rotationControllerCR(rotationTestK1, map[string]string{servicecredentialbinding.ForceRotationKey: rotationTestK1})
	cr.Spec.ForProvider.Type = "key"
	cr.Spec.WriteConnectionSecretToReference = nil
	store := newRotationStoredClient(cr)
	cf := &fake.MockServiceCredentialBinding{}
	cf.On("Get", mock.Anything, rotationTestK1).Return(rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationSucceeded), nil).Once()
	obs, err := rotationControllerExternal(store, cf).Observe(context.Background(), cr)
	require.NoError(t, err)
	require.False(t, obs.ResourceExists, "successful observation without a configured secret is the handover boundary")
}

func TestRotationAnnotationWarnings(t *testing.T) {
	for _, tc := range []struct {
		name       string
		active     string
		target     string
		kind       string
		absent     bool
		wantWarn   bool
		wantRetire bool
	}{
		{name: "empty", target: "", kind: "key", wantWarn: true},
		{name: "boolean", target: "true", kind: "key", wantWarn: true},
		{name: "timestamp", target: "2026-10-28T10:00:00Z", kind: "key", wantWarn: true},
		{name: "app", target: rotationTestK0, kind: "app", wantWarn: true},
		{name: "valid stale GUID", active: rotationTestK1, target: rotationTestK0, kind: "key"},
		{name: "absent", absent: true, kind: "key"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			annotations := map[string]string{}
			if !tc.absent {
				annotations[servicecredentialbinding.ForceRotationKey] = tc.target
			}
			active := tc.active
			if active == "" {
				active = rotationTestK0
			}
			cr := rotationControllerCR(active, annotations)
			cr.Spec.ForProvider.Type = tc.kind
			store := newRotationStoredClient(cr)
			cf := &fake.MockServiceCredentialBinding{}
			cf.On("Get", mock.Anything, active).Return(rotationControllerBinding(active, v1alpha1.LastOperationSucceeded), nil).Once()
			cf.On("GetDetails", mock.Anything, active).Return(&cfresource.ServiceCredentialBindingDetails{Credentials: map[string]any{}}, nil).Maybe()
			recorder := &fakeRecorder{}
			c := rotationControllerExternal(store, cf)
			c.recorder = recorder

			obs, err := c.Observe(context.Background(), cr)
			require.NoError(t, err)
			assert.Equal(t, !tc.wantRetire, obs.ResourceExists)
			assert.Equal(t, tc.wantWarn, len(recorder.events) > 0, "warning should explain malformed target GUID or unsupported app rotation")
			if tc.wantWarn && len(recorder.events) > 0 {
				assert.Equal(t, event.TypeWarning, recorder.events[0].Type)
				assert.True(t, strings.Contains(strings.ToLower(recorder.events[0].Message), "key"), "warning must identify the key-only rotation contract")
			}
			if !tc.wantRetire {
				assert.Empty(t, store.get().Status.AtProvider.RetiredKeys)
			}
			cf.AssertExpectations(t)
		})
	}
}

func TestRotationConnectionPublisherBoundary(t *testing.T) {
	for _, tc := range []struct {
		name    string
		asJSON  bool
		empty   bool
		initial *corev1.Secret
		wantNew bool
	}{
		{name: "flattened details", initial: nil, wantNew: true},
		{name: "JSON details", asJSON: true, initial: nil, wantNew: true},
		{name: "empty successfully retrieved details", empty: true, wantNew: true},
		{name: "unchanged correct details", initial: &corev1.Secret{Data: map[string][]byte{"username": []byte("user"), "password": []byte("secret")}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, corev1.AddToScheme(scheme))
			require.NoError(t, v1alpha1.AddToScheme(scheme))
			cr := rotationControllerCR(rotationTestK1, nil)
			cr.UID = types.UID("rotation-test-owner")
			cr.Spec.ConnectionDetailsAsJSON = tc.asJSON
			cr.Spec.WriteConnectionSecretToReference = &xpv2.SecretReference{Name: "rotation-credentials", Namespace: "rotation-test"}
			objects := []runtime.Object{cr}
			if tc.initial != nil {
				secret := tc.initial.DeepCopy()
				secret.Name, secret.Namespace = "rotation-credentials", "rotation-test"
				secret.OwnerReferences = []metav1.OwnerReference{{APIVersion: v1alpha1.CRDGroupVersion.String(), Kind: v1alpha1.ServiceCredentialBindingKind, UID: cr.UID, Controller: ptr.To(true)}}
				objects = append(objects, secret)
			}
			kube := k8sfake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objects...).Build()
			publisher := managed.NewAPISecretPublisher(kube, scheme)
			details := managed.ConnectionDetails{"username": []byte("user"), "password": []byte("secret")}
			if tc.empty {
				details = managed.ConnectionDetails{}
			}
			if tc.asJSON {
				encoded, marshalErr := json.Marshal(map[string]string{"username": "user", "password": "secret"})
				require.NoError(t, marshalErr)
				details = managed.ConnectionDetails{"credentials": encoded}
			}
			published, err := publisher.PublishConnection(context.Background(), cr, details)
			require.NoError(t, err)
			require.Equal(t, tc.wantNew, published)
			secret := &corev1.Secret{}
			require.NoError(t, kube.Get(context.Background(), types.NamespacedName{Namespace: "rotation-test", Name: "rotation-credentials"}, secret))
			require.True(t, rotationCredentialDataEqual(secret.Data, details), "published credential data mismatch")
		})
	}

	t.Run("changed secret reference is honored", func(t *testing.T) {
		scheme := runtime.NewScheme()
		require.NoError(t, corev1.AddToScheme(scheme))
		require.NoError(t, v1alpha1.AddToScheme(scheme))
		cr := rotationControllerCR(rotationTestK1, nil)
		cr.UID = types.UID("rotation-test-owner")
		cr.Spec.WriteConnectionSecretToReference = &xpv2.SecretReference{Name: "new-credentials", Namespace: "rotation-test"}
		kube := k8sfake.NewClientBuilder().WithScheme(scheme).Build()
		published, err := managed.NewAPISecretPublisher(kube, scheme).PublishConnection(context.Background(), cr, managed.ConnectionDetails{"username": []byte("user")})
		require.NoError(t, err)
		require.True(t, published)
		secret := &corev1.Secret{}
		require.NoError(t, kube.Get(context.Background(), types.NamespacedName{Namespace: "rotation-test", Name: "new-credentials"}, secret))
		require.True(t, bytes.Equal(secret.Data["username"], []byte("user")), "new secret reference did not receive current details")
	})

	t.Run("foreign owner is not overwritten", func(t *testing.T) {
		scheme := runtime.NewScheme()
		require.NoError(t, corev1.AddToScheme(scheme))
		require.NoError(t, v1alpha1.AddToScheme(scheme))
		cr := rotationControllerCR(rotationTestK1, nil)
		cr.UID = types.UID("rotation-test-owner")
		cr.Spec.WriteConnectionSecretToReference = &xpv2.SecretReference{Name: "rotation-credentials", Namespace: "rotation-test"}
		foreign := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "rotation-credentials", Namespace: "rotation-test", OwnerReferences: []metav1.OwnerReference{{UID: types.UID("other-owner"), Controller: ptr.To(true)}}}}
		kube := k8sfake.NewClientBuilder().WithScheme(scheme).WithObjects(foreign).Build()
		_, err := managed.NewAPISecretPublisher(kube, scheme).PublishConnection(context.Background(), cr, managed.ConnectionDetails{"username": []byte("user")})
		require.Error(t, err, "a foreign-owned Secret must not be overwritten")
	})
}

// TestRotationConnectionDetailsRetrievalErrorIsNotHandover records the current
// controller behavior: GetConnectionDetails swallows retrieval errors. The
// handover contract must not mistake the resulting nil details for publication.
func TestRotationConnectionDetailsRetrievalErrorIsNotHandover(t *testing.T) {
	cr := rotationControllerCR(rotationTestK1, map[string]string{servicecredentialbinding.ForceRotationKey: rotationTestK0})
	cr.Spec.ForProvider.Type = "key"
	store := newRotationStoredClient(cr)
	cf := &fake.MockServiceCredentialBinding{}
	cf.On("Get", mock.Anything, rotationTestK1).Return(rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationSucceeded), nil).Once()
	cf.On("GetDetails", mock.Anything, rotationTestK1).Return((*cfresource.ServiceCredentialBindingDetails)(nil), errors.New("unavailable")).Once()
	obs, err := rotationControllerExternal(store, cf).Observe(context.Background(), cr)
	require.NoError(t, err)
	assert.True(t, obs.ResourceExists)
	assert.Nil(t, obs.ConnectionDetails, "nil details after retrieval failure cannot prove successful secret publication")
	assert.Empty(t, store.get().Status.AtProvider.RetiredKeys, "failed detail retrieval must not cause force retirement")
}

func rotationHandoverKube(t *testing.T, cr *v1alpha1.ServiceCredentialBinding, withOldSecret bool) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, v1alpha1.AddToScheme(scheme))
	objects := []client.Object{cr}
	if withOldSecret {
		objects = append(objects, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Name: "rotation-credentials", Namespace: "rotation-test",
			OwnerReferences: []metav1.OwnerReference{{APIVersion: v1alpha1.CRDGroupVersion.String(), Kind: v1alpha1.ServiceCredentialBindingKind, UID: cr.UID, Controller: ptr.To(true)}},
		}, Data: map[string][]byte{"username": []byte("old")}})
	}
	return k8sfake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&v1alpha1.ServiceCredentialBinding{}).WithObjects(objects...).Build()
}

func rotationCredentialDataEqual(actual map[string][]byte, expected managed.ConnectionDetails) bool {
	if len(actual) != len(expected) {
		return false
	}
	for key, value := range expected {
		if !bytes.Equal(actual[key], value) {
			return false
		}
	}
	return true
}
