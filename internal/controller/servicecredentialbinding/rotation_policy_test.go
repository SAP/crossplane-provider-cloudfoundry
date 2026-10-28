package servicecredentialbinding

import (
	"context"
	"testing"
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/fake"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/servicecredentialbinding"
)

type rotationTestManager struct {
	manager.Manager
	kube   client.Client
	scheme *runtime.Scheme
}

func (m *rotationTestManager) GetClient() client.Client   { return m.kube }
func (m *rotationTestManager) GetScheme() *runtime.Scheme { return m.scheme }

func TestRotationEffectiveLegacyPolicyMatrix(t *testing.T) {
	cases := []struct {
		name          string
		policies      xpv2.ManagementPolicies
		deletion      xpv2.DeletionPolicy
		wantCreate    bool
		wantUpdate    bool
		wantDelete    bool
		wantOnlyWatch bool
		wantPause     bool
		wantErr       bool
	}{
		{name: "enabled wildcard", policies: xpv2.ManagementPolicies{xpv2.ManagementActionAll}, deletion: xpv2.DeletionDelete, wantCreate: true, wantUpdate: true, wantDelete: true},
		{name: "observe only", policies: xpv2.ManagementPolicies{xpv2.ManagementActionObserve}, deletion: xpv2.DeletionOrphan, wantOnlyWatch: true},
		{name: "observe and create without update", policies: xpv2.ManagementPolicies{xpv2.ManagementActionObserve, xpv2.ManagementActionCreate}, deletion: xpv2.DeletionOrphan, wantCreate: true},
		{name: "update only", policies: xpv2.ManagementPolicies{xpv2.ManagementActionObserve, xpv2.ManagementActionUpdate}, deletion: xpv2.DeletionOrphan, wantUpdate: true},
		{name: "empty enabled policy pauses", policies: xpv2.ManagementPolicies{}, deletion: xpv2.DeletionOrphan, wantPause: true},
		{name: "disabled feature uses legacy delete", policies: xpv2.ManagementPolicies{}, deletion: xpv2.DeletionOrphan, wantCreate: true, wantUpdate: true},
		{name: "disabled feature rejects non-default policy", policies: xpv2.ManagementPolicies{xpv2.ManagementActionObserve}, deletion: xpv2.DeletionOrphan, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cr := rotationControllerCR(rotationTestK0, nil)
			cr.Spec.ManagementPolicies = tc.policies
			cr.Spec.DeletionPolicy = tc.deletion
			enabled := tc.name != "disabled feature uses legacy delete" && tc.name != "disabled feature rejects non-default policy"
			resolver := managed.NewLegacyManagementPoliciesResolver(enabled, cr.Spec.ManagementPolicies, cr.Spec.DeletionPolicy)
			if tc.wantErr {
				require.Error(t, resolver.Validate())
				return
			}
			require.NoError(t, resolver.Validate())
			require.Equal(t, tc.wantCreate, resolver.ShouldCreate())
			require.Equal(t, tc.wantUpdate, resolver.ShouldUpdate())
			require.Equal(t, tc.wantDelete, resolver.ShouldDelete())
			require.Equal(t, tc.wantOnlyWatch, resolver.ShouldOnlyObserve())
			require.Equal(t, tc.wantPause, resolver.IsPaused())
		})
	}
}

func TestRotationRuntimeObserveOnlyDoesNotConsumeForceRequest(t *testing.T) {
	cr := rotationControllerCR(rotationTestK0, map[string]string{servicecredentialbinding.ForceRotationKey: rotationTestK0})
	cr.Spec.ManagementPolicies = xpv2.ManagementPolicies{xpv2.ManagementActionObserve}
	cr.Spec.DeletionPolicy = xpv2.DeletionOrphan
	cr.Finalizers = []string{managed.FinalizerName}
	store, scheme := rotationRuntimeStore(t, cr)
	cf := &fake.MockServiceCredentialBinding{}
	cf.On("Get", mock.Anything, rotationTestK0).Return(rotationControllerBinding(rotationTestK0, v1alpha1.LastOperationSucceeded), nil).Once()
	external := rotationRuntimeExternal(store, cf)
	reconciler := rotationRuntimeReconciler(t, store, scheme, external, true)

	_, err := reconciler.Reconcile(context.Background(), reconcile.Request{NamespacedName: types.NamespacedName{Name: cr.Name}})
	require.NoError(t, err)
	persisted := &v1alpha1.ServiceCredentialBinding{}
	require.NoError(t, store.Get(context.Background(), types.NamespacedName{Name: cr.Name}, persisted))
	require.Empty(t, persisted.Status.AtProvider.RetiredKeys, "Observe-only must not durably consume a force request")
	cf.AssertExpectations(t)
}

func TestRotationRuntimePublishesBeforeUpdate(t *testing.T) {
	cr := rotationControllerCR(rotationTestK1, nil)
	cr.UID = types.UID("rotation-test-owner")
	cr.Spec.WriteConnectionSecretToReference = &xpv2.SecretReference{Name: "rotation-credentials", Namespace: "rotation-test"}
	cr.Spec.ManagementPolicies = xpv2.ManagementPolicies{xpv2.ManagementActionObserve, xpv2.ManagementActionUpdate}
	cr.Spec.DeletionPolicy = xpv2.DeletionOrphan
	cr.Finalizers = []string{managed.FinalizerName}
	store, scheme := rotationRuntimeStore(t, cr)
	ext := &rotationPublishingExternal{kube: store, resourceExists: true}
	reconciler := rotationRuntimeReconciler(t, store, scheme, ext, true)

	_, err := reconciler.Reconcile(context.Background(), reconcile.Request{NamespacedName: types.NamespacedName{Name: cr.Name}})
	require.NoError(t, err)
	require.True(t, ext.secretPresentBeforeUpdate, "runtime must publish observed details before invoking Update")
}

type rotationPublishingExternal struct {
	kube                      client.Client
	resourceExists            bool
	createCalled              bool
	secretPresentBeforeUpdate bool
}

func (e *rotationPublishingExternal) Observe(context.Context, resource.Managed) (managed.ExternalObservation, error) {
	return managed.ExternalObservation{
		ResourceExists:   e.resourceExists,
		ResourceUpToDate: false,
		ConnectionDetails: managed.ConnectionDetails{
			"username": []byte("published-user"),
		},
	}, nil
}

func (e *rotationPublishingExternal) Create(context.Context, resource.Managed) (managed.ExternalCreation, error) {
	e.createCalled = true
	return managed.ExternalCreation{}, nil
}

func (e *rotationPublishingExternal) Update(ctx context.Context, _ resource.Managed) (managed.ExternalUpdate, error) {
	secret := &corev1.Secret{}
	err := e.kube.Get(ctx, types.NamespacedName{Namespace: "rotation-test", Name: "rotation-credentials"}, secret)
	e.secretPresentBeforeUpdate = err == nil && string(secret.Data["username"]) == "published-user"
	return managed.ExternalUpdate{}, err
}

func (e *rotationPublishingExternal) Delete(context.Context, resource.Managed) (managed.ExternalDelete, error) {
	return managed.ExternalDelete{}, nil
}

func (e *rotationPublishingExternal) Disconnect(context.Context) error { return nil }

func TestRotationRuntimeCreateWithoutUpdatePolicy(t *testing.T) {
	cr := rotationControllerCR(rotationTestK0, nil)
	cr.Spec.ManagementPolicies = xpv2.ManagementPolicies{xpv2.ManagementActionObserve, xpv2.ManagementActionCreate}
	cr.Spec.DeletionPolicy = xpv2.DeletionOrphan
	cr.Finalizers = []string{managed.FinalizerName}
	store, scheme := rotationRuntimeStore(t, cr)
	ext := &rotationPublishingExternal{kube: store, resourceExists: false}
	reconciler := rotationRuntimeReconciler(t, store, scheme, ext, true)

	_, err := reconciler.Reconcile(context.Background(), reconcile.Request{NamespacedName: types.NamespacedName{Name: cr.Name}})
	require.NoError(t, err)
	require.True(t, ext.createCalled, "Create permission must be honored without Update permission")
	require.False(t, ext.secretPresentBeforeUpdate, "Update must remain disallowed")
}

func TestRotationRuntimeDisabledPolicyRejectsBeforeExternalWork(t *testing.T) {
	cr := rotationControllerCR(rotationTestK0, nil)
	cr.Spec.ManagementPolicies = xpv2.ManagementPolicies{xpv2.ManagementActionObserve}
	cr.Finalizers = []string{managed.FinalizerName}
	store, scheme := rotationRuntimeStore(t, cr)
	cf := &fake.MockServiceCredentialBinding{}
	reconciler := rotationRuntimeReconciler(t, store, scheme, rotationRuntimeExternal(store, cf), false)

	_, err := reconciler.Reconcile(context.Background(), reconcile.Request{NamespacedName: types.NamespacedName{Name: cr.Name}})
	require.NoError(t, err, "invalid disabled-feature policy is reported through status, before external connection")
	cf.AssertNotCalled(t, "Get", mock.Anything, mock.Anything)
}

func TestRotationRuntimePolicyPauseDoesNotConnect(t *testing.T) {
	cr := rotationControllerCR(rotationTestK0, nil)
	cr.Spec.ManagementPolicies = xpv2.ManagementPolicies{}
	cr.Finalizers = []string{managed.FinalizerName}
	store, scheme := rotationRuntimeStore(t, cr)
	cf := &fake.MockServiceCredentialBinding{}
	reconciler := rotationRuntimeReconciler(t, store, scheme, rotationRuntimeExternal(store, cf), true)

	_, err := reconciler.Reconcile(context.Background(), reconcile.Request{NamespacedName: types.NamespacedName{Name: cr.Name}})
	require.NoError(t, err)
	cf.AssertNotCalled(t, "Get", mock.Anything, mock.Anything)
}

func TestRotationRuntimeOrphanDoesNotDeleteRetiredKeys(t *testing.T) {
	cr := rotationControllerCR(rotationTestK0, nil)
	cr.Spec.ManagementPolicies = xpv2.ManagementPolicies{xpv2.ManagementActionObserve}
	cr.Spec.DeletionPolicy = xpv2.DeletionOrphan
	cr.Finalizers = []string{managed.FinalizerName}
	now := metav1.NewTime(time.Now())
	cr.SetDeletionTimestamp(&now)
	cr.Status.AtProvider.RetiredKeys = []*v1alpha1.SCBResource{{GUID: rotationTestK1}}
	store, scheme := rotationRuntimeStore(t, cr)
	cf := &fake.MockServiceCredentialBinding{}
	external := rotationRuntimeExternal(store, cf)
	reconciler := rotationRuntimeReconciler(t, store, scheme, external, true)

	_, err := reconciler.Reconcile(context.Background(), reconcile.Request{NamespacedName: types.NamespacedName{Name: cr.Name}})
	require.NoError(t, err)
	cf.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
	cf.AssertNotCalled(t, "Get", mock.Anything, mock.Anything)
}

func rotationRuntimeExternal(kube client.Client, cf *fake.MockServiceCredentialBinding) *external {
	c := &external{kube: kube, scbClient: cf, keyRotator: &servicecredentialbinding.SCBKeyRotator{SCBClient: cf}, maxCreateAttempts: DefaultMaxCreateAttempts}
	c.observationStateHandler = c
	return c
}

func rotationRuntimeStore(t *testing.T, cr *v1alpha1.ServiceCredentialBinding) (client.Client, *runtime.Scheme) {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, v1alpha1.AddToScheme(scheme))
	return k8sfake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&v1alpha1.ServiceCredentialBinding{}).WithObjects(cr).Build(), scheme
}

func rotationRuntimeReconciler(t *testing.T, kube client.Client, scheme *runtime.Scheme, external managed.ExternalClient, enablePolicies bool) *managed.Reconciler {
	t.Helper()
	options := []managed.ReconcilerOption{
		managed.WithInitializers(),
		managed.WithExternalConnector(rotationTestExternalConnector{external: external}),
		managed.WithPollInterval(time.Millisecond),
	}
	if enablePolicies {
		options = append(options, managed.WithManagementPolicies())
	}
	return managed.NewReconciler(&rotationTestManager{kube: kube, scheme: scheme}, resource.ManagedKind(v1alpha1.ServiceCredentialBindingGroupVersionKind), options...)
}

type rotationTestExternalConnector struct{ external managed.ExternalClient }

func (c rotationTestExternalConnector) Connect(context.Context, resource.Managed) (managed.ExternalClient, error) {
	return c.external, nil
}
