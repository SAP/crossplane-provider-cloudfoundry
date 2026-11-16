package servicecredentialbinding

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
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
	cr.Spec.ManagementPolicies = xpv2.ManagementPolicies{xpv2.ManagementActionAll}
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
			require.NoError(t, err)
			assert.True(t, obs.ResourceExists, "a replacement cannot be retired before handover")
			persisted := &v1alpha1.ServiceCredentialBinding{}
			require.NoError(t, kube.Get(context.Background(), types.NamespacedName{Name: cr.Name}, persisted))
			assert.Len(t, persisted.Status.AtProvider.RetiredKeys, 1)
			if len(persisted.Status.AtProvider.RetiredKeys) > 0 {
				assert.Equal(t, rotationTestK0, persisted.Status.AtProvider.RetiredKeys[0].GUID)
			}
			cf.AssertExpectations(t)
		})
	}
}

func TestRotationHandoverAllowsAdditionalSecretKeys(t *testing.T) {
	for _, tc := range []struct {
		name    string
		data    map[string][]byte
		details managed.ConnectionDetails
		want    bool
	}{
		{name: "additional old format key", data: map[string][]byte{"username": []byte("new"), "credentials": []byte(`{"username":"old"}`)}, details: managed.ConnectionDetails{"username": []byte("new")}, want: true},
		{name: "missing expected key", data: map[string][]byte{"credentials": []byte(`{"username":"new"}`)}, details: managed.ConnectionDetails{"username": []byte("new")}},
		{name: "wrong expected value", data: map[string][]byte{"username": []byte("old"), "credentials": []byte(`{"username":"old"}`)}, details: managed.ConnectionDetails{"username": []byte("new")}},
		{name: "empty expected key absent", data: map[string][]byte{"old": []byte("value")}, details: managed.ConnectionDetails{"username": []byte{}}},
		{name: "empty expected key present", data: map[string][]byte{"username": {}, "old": []byte("value")}, details: managed.ConnectionDetails{"username": []byte{}}, want: true},
		{name: "empty expected map", data: map[string][]byte{"old": []byte("value")}, details: managed.ConnectionDetails{}, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cr := rotationHandoverCR(rotationTestK1, "__absent__", "key")
			kube := rotationHandoverKube(t, cr, false)
			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "rotation-credentials", Namespace: "rotation-test", OwnerReferences: []metav1.OwnerReference{{APIVersion: v1alpha1.CRDGroupVersion.String(), Kind: v1alpha1.ServiceCredentialBindingKind, Name: cr.Name, UID: cr.UID, Controller: ptr.To(true)}}}, Type: resource.SecretTypeConnection, Data: tc.data}
			require.NoError(t, kube.Create(context.Background(), secret))
			c := rotationRuntimeExternal(kube, &fake.MockServiceCredentialBinding{})
			assert.Equal(t, tc.want, c.confirmSecretHandover(context.Background(), cr, tc.details))
		})
	}
}

func TestRotationHandoverAncillaryOwners(t *testing.T) {
	cr := rotationHandoverCR(rotationTestK1, "__absent__", "key")
	kube := rotationHandoverKube(t, cr, false)
	expected := resource.ConnectionSecretFor(cr, v1alpha1.ServiceCredentialBindingGroupVersionKind)
	controller := expected.OwnerReferences[0]
	ancillary := metav1.OwnerReference{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		Name:       "independent-owner",
		UID:        types.UID("independent-owner-uid"),
		Controller: ptr.To(false),
	}

	for _, tc := range []struct {
		name         string
		owners       []metav1.OwnerReference
		secretType   corev1.SecretType
		secretName   string
		wantHandover bool
	}{
		{name: "ancillary owner after controller", owners: []metav1.OwnerReference{controller, ancillary}, secretType: resource.SecretTypeConnection, secretName: expected.Name, wantHandover: true},
		{name: "ancillary owner before controller", owners: []metav1.OwnerReference{ancillary, controller}, secretType: resource.SecretTypeConnection, secretName: expected.Name, wantHandover: true},
		{name: "missing controller", owners: []metav1.OwnerReference{ancillary}, secretType: resource.SecretTypeConnection, secretName: expected.Name},
		{name: "foreign controller UID", owners: []metav1.OwnerReference{func() metav1.OwnerReference { r := controller; r.UID = types.UID("foreign-controller"); return r }(), ancillary}, secretType: resource.SecretTypeConnection, secretName: expected.Name},
		{name: "foreign controller name", owners: []metav1.OwnerReference{func() metav1.OwnerReference { r := controller; r.Name = "other-binding"; return r }(), ancillary}, secretType: resource.SecretTypeConnection, secretName: expected.Name},
		{name: "foreign controller kind", owners: []metav1.OwnerReference{func() metav1.OwnerReference { r := controller; r.Kind = "Other"; return r }(), ancillary}, secretType: resource.SecretTypeConnection, secretName: expected.Name},
		{name: "foreign controller API version", owners: []metav1.OwnerReference{func() metav1.OwnerReference { r := controller; r.APIVersion = "other/v1"; return r }(), ancillary}, secretType: resource.SecretTypeConnection, secretName: expected.Name},
		{name: "non-controller binding owner", owners: []metav1.OwnerReference{func() metav1.OwnerReference { r := controller; r.Controller = ptr.To(false); return r }(), ancillary}, secretType: resource.SecretTypeConnection, secretName: expected.Name},
		{name: "wrong Secret type", owners: []metav1.OwnerReference{controller, ancillary}, secretType: corev1.SecretTypeOpaque, secretName: expected.Name},
		{name: "wrong Secret reference", owners: []metav1.OwnerReference{controller, ancillary}, secretType: resource.SecretTypeConnection, secretName: "other-secret"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: tc.secretName, Namespace: expected.Namespace, OwnerReferences: tc.owners},
				Type:       tc.secretType,
				Data:       map[string][]byte{"username": []byte("current"), "extra": []byte("preserved")},
			}
			require.NoError(t, kube.Create(context.Background(), secret))
			c := rotationRuntimeExternal(kube, &fake.MockServiceCredentialBinding{})
			got := c.confirmSecretHandover(context.Background(), cr, managed.ConnectionDetails{"username": []byte("current")})
			assert.Equal(t, tc.wantHandover, got)
			assert.Equal(t, tc.owners, secret.OwnerReferences, "handover checks must not mutate ownership")
			require.NoError(t, kube.Delete(context.Background(), secret))
		})
	}
}

func TestRotationPublisherNoopPreservesAncillaryOwners(t *testing.T) {
	ctx := context.Background()
	scheme := mustRotationScheme(t)
	cr := rotationControllerCR(rotationTestK1, nil)
	cr.UID = types.UID("rotation-test-owner")
	cr.Spec.WriteConnectionSecretToReference = &xpv2.SecretReference{Name: "rotation-credentials", Namespace: "rotation-test"}
	kube := k8sfake.NewClientBuilder().WithScheme(scheme).WithObjects(cr).Build()
	publisher := managed.NewAPISecretPublisher(kube, scheme)
	details := managed.ConnectionDetails{"username": []byte("current")}
	_, err := publisher.PublishConnection(ctx, cr, details)
	require.NoError(t, err)

	secret := &corev1.Secret{}
	key := types.NamespacedName{Namespace: "rotation-test", Name: "rotation-credentials"}
	require.NoError(t, kube.Get(ctx, key, secret))
	ancillary := metav1.OwnerReference{APIVersion: "v1", Kind: "ConfigMap", Name: "independent-owner", UID: types.UID("independent-owner-uid"), Controller: ptr.To(false)}
	secret.OwnerReferences = append(secret.OwnerReferences, ancillary)
	require.NoError(t, kube.Update(ctx, secret))

	published, err := publisher.PublishConnection(ctx, cr, details)
	require.NoError(t, err)
	assert.False(t, published, "unchanged credentials are a publisher no-op")
	require.NoError(t, kube.Get(ctx, key, secret))
	assert.Contains(t, secret.OwnerReferences, ancillary)
	assert.True(t, rotationRuntimeExternal(kube, &fake.MockServiceCredentialBinding{}).confirmSecretHandover(ctx, cr, details))
}

func TestRotationHandoverSecretMismatchAndReadErrors(t *testing.T) {
	for _, tc := range []struct {
		name      string
		hasSecret bool
		readerErr bool
	}{
		{name: "missing secret"},
		{name: "secret has old credentials", hasSecret: true},
		{name: "secret read error", hasSecret: true, readerErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cr := rotationHandoverCR(rotationTestK1, rotationTestK1, "key")
			cf := &fake.MockServiceCredentialBinding{}
			cf.On("Get", mock.Anything, rotationTestK1).Return(rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationSucceeded), nil).Once()
			cf.On("GetDetails", mock.Anything, rotationTestK1).Return(&cfresource.ServiceCredentialBindingDetails{Credentials: map[string]any{"username": "new"}}, nil).Maybe()
			kube := rotationHandoverKube(t, cr, tc.hasSecret)
			external := rotationRuntimeExternal(kube, cf)
			if tc.readerErr {
				external.reader = rotationReadErrorReader{Reader: kube, err: errors.New("secret read failed")}
			}
			obs, err := external.Observe(context.Background(), cr)
			require.NoError(t, err)
			assert.True(t, obs.ResourceExists)
			persisted := &v1alpha1.ServiceCredentialBinding{}
			require.NoError(t, kube.Get(context.Background(), types.NamespacedName{Name: cr.Name}, persisted))
			assert.Len(t, persisted.Status.AtProvider.RetiredKeys, 1)
			cf.AssertExpectations(t)
		})
	}
}

func TestRotationDeferredIntentRemovedBeforeHandover(t *testing.T) {
	cr := rotationHandoverCR(rotationTestK1, rotationTestK1, "key")
	cf := &fake.MockServiceCredentialBinding{}
	cf.On("Get", mock.Anything, rotationTestK1).Return(rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationInProgress), nil).Once()
	cf.On("Get", mock.Anything, rotationTestK1).Return(rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationSucceeded), nil).Twice()
	cf.On("GetDetails", mock.Anything, rotationTestK1).Return(&cfresource.ServiceCredentialBindingDetails{Credentials: map[string]any{"username": "new"}}, nil).Twice()
	kube := rotationHandoverKube(t, cr, true)
	external := rotationRuntimeExternal(kube, cf)

	first, err := external.Observe(context.Background(), cr)
	require.NoError(t, err)
	require.True(t, first.ResourceExists)
	persisted := &v1alpha1.ServiceCredentialBinding{}
	require.NoError(t, kube.Get(context.Background(), types.NamespacedName{Name: cr.Name}, persisted))
	require.Len(t, persisted.Status.AtProvider.RetiredKeys, 1)
	require.Equal(t, rotationTestK0, persisted.Status.AtProvider.RetiredKeys[0].GUID, "persisted retirement must survive removal of deferred intent")

	delete(persisted.Annotations, servicecredentialbinding.ForceRotationKey)
	require.NoError(t, kube.Update(context.Background(), persisted))
	secondCR := &v1alpha1.ServiceCredentialBinding{}
	require.NoError(t, kube.Get(context.Background(), types.NamespacedName{Name: cr.Name}, secondCR))
	second, err := external.Observe(context.Background(), secondCR)
	require.NoError(t, err)
	require.True(t, second.ResourceExists, "old Secret data must defer handover")

	publisher := managed.NewAPISecretPublisher(kube, mustRotationScheme(t))
	_, err = publisher.PublishConnection(context.Background(), secondCR, second.ConnectionDetails)
	require.NoError(t, err)
	finalCR := &v1alpha1.ServiceCredentialBinding{}
	require.NoError(t, kube.Get(context.Background(), types.NamespacedName{Name: cr.Name}, finalCR))
	final, err := external.Observe(context.Background(), finalCR)
	require.NoError(t, err)
	require.True(t, final.ResourceExists, "removing unaccepted intent must not retire the handed-over active generation")
	finalCR = &v1alpha1.ServiceCredentialBinding{}
	require.NoError(t, kube.Get(context.Background(), types.NamespacedName{Name: cr.Name}, finalCR))
	require.Len(t, finalCR.Status.AtProvider.RetiredKeys, 1)
	require.Equal(t, rotationTestK0, finalCR.Status.AtProvider.RetiredKeys[0].GUID)
	cf.AssertExpectations(t)
}

type rotationReadErrorReader struct {
	client.Reader
	err error
}

func (r rotationReadErrorReader) Get(context.Context, types.NamespacedName, client.Object, ...client.GetOption) error {
	return r.err
}

func TestRotationUpdateReturnsFreshConnectionDetails(t *testing.T) {
	cr := rotationControllerCR(rotationTestK1, nil)
	cr.Spec.ManagementPolicies = xpv2.ManagementPolicies{xpv2.ManagementActionAll}
	cr.UID = types.UID("rotation-test-owner")
	cr.Spec.WriteConnectionSecretToReference = &xpv2.SecretReference{Name: "rotation-credentials", Namespace: "rotation-test"}
	kube := rotationHandoverKube(t, cr, true)
	cf := &fake.MockServiceCredentialBinding{}
	cf.On("Update", mock.Anything, rotationTestK1, mock.Anything).Return(rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationSucceeded), nil).Once()
	cf.On("Get", mock.Anything, rotationTestK1).Return(rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationSucceeded), nil).Once()
	cf.On("GetDetails", mock.Anything, rotationTestK1).Return(&cfresource.ServiceCredentialBindingDetails{Credentials: map[string]any{"username": "new"}}, nil).Once()

	update, err := rotationRuntimeExternal(kube, cf).Update(context.Background(), cr)
	require.NoError(t, err)
	require.Equal(t, managed.ConnectionDetails{"username": []byte("new")}, update.ConnectionDetails,
		"runtime republishes Update.ConnectionDetails after metadata/cleanup work")
	cf.AssertExpectations(t)
}

func TestRotationRuntimePublisherReadbackEnablesHandover(t *testing.T) {
	ctx := context.Background()
	cr := rotationHandoverCR(rotationTestK1, rotationTestK1, "key")
	cf := &fake.MockServiceCredentialBinding{}
	cf.On("Get", mock.Anything, rotationTestK1).Return(rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationSucceeded), nil).Twice()
	cf.On("GetDetails", mock.Anything, rotationTestK1).Return(&cfresource.ServiceCredentialBindingDetails{Credentials: map[string]any{"username": "new"}}, nil).Twice()
	kube := rotationHandoverKube(t, cr, false)
	c := rotationRuntimeExternal(kube, cf)
	handover := time.Date(2026, 10, 28, 9, 30, 0, 0, time.UTC)
	c.nowFn = func() time.Time { return handover }
	c.keyRotator.(*servicecredentialbinding.SCBKeyRotator).Now = func() time.Time { return handover }

	first, err := c.Observe(ctx, cr)
	require.NoError(t, err)
	require.True(t, first.ResourceExists, "missing Secret must defer a new retirement")
	require.Len(t, cr.Status.AtProvider.RetiredKeys, 1, "the active K1 must not be retired before publication")

	publisher := managed.NewAPISecretPublisher(kube, mustRotationScheme(t))
	_, err = publisher.PublishConnection(ctx, cr, first.ConnectionDetails)
	require.NoError(t, err)
	require.NoError(t, kube.Status().Update(ctx, cr))
	reloaded := &v1alpha1.ServiceCredentialBinding{}
	require.NoError(t, kube.Get(ctx, types.NamespacedName{Name: cr.Name}, reloaded))

	second, err := c.Observe(ctx, reloaded)
	require.NoError(t, err)
	require.False(t, second.ResourceExists, "fresh read-back of publisher output must enable the pending current-GUID request")
	require.Len(t, reloaded.Status.AtProvider.RetiredKeys, 2)
	require.Equal(t, rotationTestK1, reloaded.Status.AtProvider.RetiredKeys[1].GUID)
	require.NotNil(t, reloaded.Status.AtProvider.RetiredKeys[0].DeleteAfter)
	require.True(t, reloaded.Status.AtProvider.RetiredKeys[0].DeleteAfter.Time.Equal(handover.Add(48*time.Hour)))
	cf.AssertExpectations(t)
}

func mustRotationScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, v1alpha1.AddToScheme(scheme))
	return scheme
}

func TestRotationConfirmedUnchangedSecretPersistsDeadlineAndAllowsNextRotation(t *testing.T) {
	cr := rotationHandoverCR(rotationTestK1, rotationTestK1, "key")
	cf := &fake.MockServiceCredentialBinding{}
	cf.On("Get", mock.Anything, rotationTestK1).Return(rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationSucceeded), nil).Once()
	cf.On("GetDetails", mock.Anything, rotationTestK1).Return(&cfresource.ServiceCredentialBindingDetails{Credentials: map[string]any{"username": "new"}}, nil).Once()
	kube := rotationHandoverKube(t, cr, true)
	secret := &corev1.Secret{}
	require.NoError(t, kube.Get(context.Background(), types.NamespacedName{Namespace: "rotation-test", Name: "rotation-credentials"}, secret))
	secret.Data = map[string][]byte{"username": []byte("new")}
	require.NoError(t, kube.Update(context.Background(), secret))
	c := rotationRuntimeExternal(kube, cf)
	expectedOwner := resource.ConnectionSecretFor(cr, v1alpha1.ServiceCredentialBindingGroupVersionKind)
	require.Equal(t, expectedOwner.Type, secret.Type)
	require.True(t, sameControllerOwner(secret, expectedOwner))
	require.True(t, connectionDataContains(secret.Data, managed.ConnectionDetails{"username": []byte("new")}))
	binding := rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationSucceeded)
	require.Equal(t, v1alpha1.LastOperationSucceeded, binding.LastOperation.State)
	require.True(t, c.confirmHandover(context.Background(), cr, rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationSucceeded), managed.ConnectionDetails{"username": []byte("new")}), "an unchanged, correctly owned Secret must confirm handover")

	obs, err := c.Observe(context.Background(), cr)
	require.NoError(t, err)
	require.False(t, obs.ResourceExists, "a correctly published, unchanged Secret satisfies handover and permits a new current-GUID request")
	require.Len(t, cr.Status.AtProvider.RetiredKeys, 2)
	require.Equal(t, rotationTestK0, cr.Status.AtProvider.RetiredKeys[0].GUID)
	require.NotNil(t, cr.Status.AtProvider.RetiredKeys[0].DeleteAfter)
	require.Equal(t, rotationTestK1, cr.Status.AtProvider.RetiredKeys[1].GUID)
	require.Nil(t, cr.Status.AtProvider.RetiredKeys[1].DeleteAfter, "the newly retired active key has not yet handed over")
	persisted := &v1alpha1.ServiceCredentialBinding{}
	require.NoError(t, kube.Get(context.Background(), types.NamespacedName{Name: cr.Name}, persisted))
	require.NotNil(t, persisted.Status.AtProvider.RetiredKeys[0].DeleteAfter, "fallback deadline must persist before replacement Create")
	cf.AssertExpectations(t)
}

func TestRotationHandoverNoSecretPositiveControl(t *testing.T) {
	cr := rotationControllerCR(rotationTestK1, map[string]string{servicecredentialbinding.ForceRotationKey: rotationTestK1})
	cr.Spec.ForProvider.Type = "key"
	cr.Spec.WriteConnectionSecretToReference = nil
	store := newRotationStoredClient(cr)
	cf := &fake.MockServiceCredentialBinding{}
	cf.On("Get", mock.Anything, rotationTestK1).Return(rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationSucceeded), nil).Once()
	cf.On("GetDetails", mock.Anything, rotationTestK1).Return(&cfresource.ServiceCredentialBindingDetails{Credentials: map[string]any{}}, nil).Once()
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
		{name: "braced GUID", target: "{" + rotationTestK0 + "}", kind: "key", wantWarn: true},
		{name: "URN GUID", target: "urn:uuid:" + rotationTestK0, kind: "key", wantWarn: true},
		{name: "unhyphenated GUID", target: strings.ReplaceAll(rotationTestK0, "-", ""), kind: "key", wantWarn: true},
		{name: "GUID with whitespace", target: " " + rotationTestK0, kind: "key", wantWarn: true},
		{name: "uppercase matching GUID", active: "abcdef01-2345-4678-9abc-def012345678", target: "ABCDEF01-2345-4678-9ABC-DEF012345678", kind: "key", wantRetire: true},
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
				assert.Contains(t, strings.ToLower(recorder.events[0].Message), "key", "warning must identify the key-only rotation contract")
			}
			if !tc.wantRetire {
				assert.Empty(t, store.get().Status.AtProvider.RetiredKeys)
			}
			cf.AssertExpectations(t)
		})
	}
}

func TestRotationPublisherFormatChangesLeaveHandoverConvergent(t *testing.T) {
	for _, tc := range []struct {
		name             string
		initialJSON      bool
		nextJSON         bool
		initialDetails   managed.ConnectionDetails
		nextDetails      managed.ConnectionDetails
		wantPreservedKey string
	}{
		{
			name: "JSON to flattened with shrinking credentials", initialJSON: true,
			initialDetails: managed.ConnectionDetails{"credentials": []byte(`{"username":"user","password":"old"}`)},
			nextDetails:    managed.ConnectionDetails{"username": []byte("user")}, wantPreservedKey: "credentials",
		},
		{
			name: "flattened to JSON with shrinking credentials", nextJSON: true,
			initialDetails: managed.ConnectionDetails{"username": []byte("user"), "password": []byte("old")},
			nextDetails:    managed.ConnectionDetails{"credentials": []byte(`{"username":"user"}`)}, wantPreservedKey: "password",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			scheme := mustRotationScheme(t)
			cr := rotationControllerCR(rotationTestK1, nil)
			cr.UID = types.UID("rotation-test-owner")
			cr.Spec.ConnectionDetailsAsJSON = tc.initialJSON
			cr.Spec.WriteConnectionSecretToReference = &xpv2.SecretReference{Name: "rotation-credentials", Namespace: "rotation-test"}
			kube := k8sfake.NewClientBuilder().WithScheme(scheme).WithObjects(cr).Build()
			publisher := managed.NewAPISecretPublisher(kube, scheme)
			if _, err := publisher.PublishConnection(ctx, cr, tc.initialDetails); err != nil {
				t.Fatalf("publish initial credential format: %v", err)
			}
			cr.Spec.ConnectionDetailsAsJSON = tc.nextJSON
			if _, err := publisher.PublishConnection(ctx, cr, tc.nextDetails); err != nil {
				t.Fatalf("publish changed credential format: %v", err)
			}
			secret := &corev1.Secret{}
			require.NoError(t, kube.Get(ctx, types.NamespacedName{Namespace: "rotation-test", Name: "rotation-credentials"}, secret))
			require.Contains(t, secret.Data, tc.wantPreservedKey)
			c := rotationRuntimeExternal(kube, &fake.MockServiceCredentialBinding{})
			require.True(t, c.confirmSecretHandover(ctx, cr, tc.nextDetails), "expected current entries must confirm despite merge-patch leftovers")
		})
	}
}

// TestRotationConnectionDetailsRetrievalErrorIsNotHandover verifies that
// required connection details cannot silently become an empty publication.
func TestRotationConnectionDetailsRetrievalErrorIsNotHandover(t *testing.T) {
	t.Run("configured secret fails observation", func(t *testing.T) {
		cr := rotationControllerCR(rotationTestK1, nil)
		cr.Spec.ForProvider.Type = "key"
		cr.Spec.WriteConnectionSecretToReference = &xpv2.SecretReference{Name: "rotation-credentials", Namespace: "rotation-test"}
		cr.Status.AtProvider.RetiredKeys = []*v1alpha1.SCBResource{{GUID: rotationTestK0, CreatedAt: &metav1.Time{Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}}}
		store := newRotationStoredClient(cr)
		cf := &fake.MockServiceCredentialBinding{}
		cf.On("Get", mock.Anything, rotationTestK1).Return(rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationSucceeded), nil).Once()
		cf.On("GetDetails", mock.Anything, rotationTestK1).Return((*cfresource.ServiceCredentialBindingDetails)(nil), errors.New("unavailable")).Once()
		obs, err := rotationControllerExternal(store, cf).Observe(context.Background(), cr)
		require.Error(t, err)
		assert.Empty(t, obs.ConnectionDetails)
		persisted := store.get()
		assert.Len(t, persisted.Status.AtProvider.RetiredKeys, 1, "failed required detail retrieval must not cause force retirement")
		assert.Nil(t, persisted.Status.AtProvider.RetiredKeys[0].DeleteAfter, "failed required detail retrieval must not establish the handover deadline")
		cf.AssertExpectations(t)
	})

	t.Run("no secret keeps optional details best effort", func(t *testing.T) {
		cr := rotationControllerCR(rotationTestK1, nil)
		cr.Spec.ForProvider.Type = "key"
		store := newRotationStoredClient(cr)
		cf := &fake.MockServiceCredentialBinding{}
		cf.On("Get", mock.Anything, rotationTestK1).Return(rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationSucceeded), nil).Once()
		cf.On("GetDetails", mock.Anything, rotationTestK1).Return((*cfresource.ServiceCredentialBindingDetails)(nil), errors.New("unavailable")).Once()
		obs, err := rotationControllerExternal(store, cf).Observe(context.Background(), cr)
		require.NoError(t, err)
		assert.True(t, obs.ResourceExists)
		assert.Empty(t, obs.ConnectionDetails)
		cf.AssertExpectations(t)
	})
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
			OwnerReferences: []metav1.OwnerReference{{APIVersion: v1alpha1.CRDGroupVersion.String(), Kind: v1alpha1.ServiceCredentialBindingKind, Name: cr.Name, UID: cr.UID, Controller: ptr.To(true)}},
		}, Type: resource.SecretTypeConnection, Data: map[string][]byte{"username": []byte("old")}})
	}
	return k8sfake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&v1alpha1.ServiceCredentialBinding{}).WithObjects(objects...).Build()
}
