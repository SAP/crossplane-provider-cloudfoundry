package servicecredentialbinding

import (
	"context"
	"testing"
	"time"

	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/fake"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/metadata"
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
		name            string
		policies        xpv2.ManagementPolicies
		deletion        xpv2.DeletionPolicy
		wantCreate      bool
		wantUpdate      bool
		wantDelete      bool
		wantOnlyWatch   bool
		wantPause       bool
		wantErr         bool
		policiesEnabled bool
	}{
		{name: "enabled wildcard", policies: xpv2.ManagementPolicies{xpv2.ManagementActionAll}, deletion: xpv2.DeletionDelete, wantCreate: true, wantUpdate: true, wantDelete: true, policiesEnabled: true},
		{name: "observe only", policies: xpv2.ManagementPolicies{xpv2.ManagementActionObserve}, deletion: xpv2.DeletionOrphan, wantOnlyWatch: true, policiesEnabled: true},
		{name: "observe and create without update", policies: xpv2.ManagementPolicies{xpv2.ManagementActionObserve, xpv2.ManagementActionCreate}, deletion: xpv2.DeletionOrphan, wantCreate: true, policiesEnabled: true},
		{name: "update only", policies: xpv2.ManagementPolicies{xpv2.ManagementActionObserve, xpv2.ManagementActionUpdate}, deletion: xpv2.DeletionOrphan, wantUpdate: true, policiesEnabled: true},
		{name: "empty enabled policy pauses", policies: xpv2.ManagementPolicies{}, deletion: xpv2.DeletionOrphan, wantPause: true, policiesEnabled: true},
		{name: "disabled feature uses legacy delete", policies: xpv2.ManagementPolicies{}, deletion: xpv2.DeletionOrphan, wantCreate: true, wantUpdate: true},
		{name: "disabled feature rejects non-default policy", policies: xpv2.ManagementPolicies{xpv2.ManagementActionObserve}, deletion: xpv2.DeletionOrphan, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cr := rotationControllerCR(rotationTestK0, nil)
			cr.Spec.ManagementPolicies = tc.policies
			cr.Spec.DeletionPolicy = tc.deletion
			external := &external{managementPoliciesEnabled: tc.policiesEnabled}
			resolver, err := external.lifecyclePolicies(cr)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantCreate, resolver.ShouldCreate())
			require.Equal(t, tc.wantUpdate, resolver.ShouldUpdate())
			require.Equal(t, tc.wantDelete, resolver.ShouldDelete())
			require.Equal(t, tc.wantOnlyWatch, resolver.ShouldOnlyObserve())
			require.Equal(t, tc.wantPause, resolver.IsPaused())
		})
	}
}

func TestRotationFallbackDeadlineInitializationWithoutCreatePolicy(t *testing.T) {
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name                      string
		policies                  xpv2.ManagementPolicies
		deletion                  xpv2.DeletionPolicy
		managementPoliciesEnabled bool
		rotation                  *v1alpha1.RotationParameters
		removeTTL                 bool
		target                    string
		wantInit                  bool
		wantCleanup               bool
	}{
		{name: "observe and update no rotation", policies: xpv2.ManagementPolicies{xpv2.ManagementActionObserve, xpv2.ManagementActionUpdate}, deletion: xpv2.DeletionOrphan, managementPoliciesEnabled: true, target: rotationTestK1, wantInit: true, wantCleanup: true},
		{name: "observe update and late initialize frequency only", policies: xpv2.ManagementPolicies{xpv2.ManagementActionObserve, xpv2.ManagementActionLateInitialize, xpv2.ManagementActionUpdate}, deletion: xpv2.DeletionOrphan, managementPoliciesEnabled: true, rotation: &v1alpha1.RotationParameters{Frequency: &metav1.Duration{Duration: 24 * time.Hour}}, target: rotationTestK1, wantInit: true, wantCleanup: true},
		{name: "observe update after ttl removal", policies: xpv2.ManagementPolicies{xpv2.ManagementActionObserve, xpv2.ManagementActionUpdate}, deletion: xpv2.DeletionOrphan, managementPoliciesEnabled: true, rotation: &v1alpha1.RotationParameters{Frequency: &metav1.Duration{Duration: 24 * time.Hour}, TTL: &metav1.Duration{Duration: 72 * time.Hour}}, removeTTL: true, target: rotationTestK1, wantInit: true, wantCleanup: true},
		{name: "wildcard policy", policies: xpv2.ManagementPolicies{xpv2.ManagementActionAll}, deletion: xpv2.DeletionDelete, managementPoliciesEnabled: true, wantInit: true, wantCleanup: true},
		{name: "create without update", policies: xpv2.ManagementPolicies{xpv2.ManagementActionObserve, xpv2.ManagementActionCreate}, deletion: xpv2.DeletionOrphan, managementPoliciesEnabled: true, wantInit: true},
		{name: "observe only", policies: xpv2.ManagementPolicies{xpv2.ManagementActionObserve}, deletion: xpv2.DeletionOrphan, managementPoliciesEnabled: true},
		{name: "enabled empty policy pauses", policies: xpv2.ManagementPolicies{}, deletion: xpv2.DeletionOrphan, managementPoliciesEnabled: true},
		{name: "legacy default policy", wantInit: true, wantCleanup: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			target := tc.target
			if target == "" {
				target = rotationTestK0
			}
			cr := rotationControllerCR(rotationTestK1, map[string]string{servicecredentialbinding.ForceRotationKey: target})
			cr.Spec.ManagementPolicies = tc.policies
			cr.Spec.DeletionPolicy = tc.deletion
			cr.Spec.ForProvider.Rotation = tc.rotation
			if tc.removeTTL {
				cr.Spec.ForProvider.Rotation.TTL = nil
			}
			cr.Finalizers = []string{managed.FinalizerName}
			cr.Status.AtProvider.CreatedAt = &metav1.Time{Time: now}
			cr.Status.AtProvider.RetiredKeys = []*v1alpha1.SCBResource{{GUID: rotationTestK0, CreatedAt: &metav1.Time{Time: now.Add(-180 * 24 * time.Hour)}}}
			store, scheme := rotationRuntimeStore(t, cr)
			cf := &fake.MockServiceCredentialBinding{}
			if tc.name != "enabled empty policy pauses" {
				binding := rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationSucceeded)
				binding.Metadata = metadata.BuildMetadata(cr, cr.Spec.ForProvider.Labels, cr.Spec.ForProvider.Annotations)
				require.True(t, servicecredentialbinding.IsUpToDate(context.Background(), cr, cr.Spec.ForProvider, *binding), "test must exercise an observed binding without metadata drift")
				cf.On("Get", mock.Anything, rotationTestK1).Return(binding, nil).Maybe()
				cf.On("Update", mock.Anything, rotationTestK1, mock.Anything).Return(binding, nil).Maybe()
			}
			external := rotationRuntimeExternal(store, cf)
			external.managementPoliciesEnabled = tc.managementPoliciesEnabled
			external.nowFn = func() time.Time { return now }
			external.keyRotator.(*servicecredentialbinding.SCBKeyRotator).Now = func() time.Time { return now }
			reconciler := rotationRuntimeReconciler(t, store, scheme, external, tc.managementPoliciesEnabled)

			request := reconcile.Request{NamespacedName: types.NamespacedName{Name: cr.Name}}
			_, err := reconciler.Reconcile(context.Background(), request)
			require.NoError(t, err)
			if tc.wantInit {
				persisted := &v1alpha1.ServiceCredentialBinding{}
				require.NoError(t, store.Get(context.Background(), types.NamespacedName{Name: cr.Name}, persisted))
				require.Len(t, persisted.Status.AtProvider.RetiredKeys, 1)
				deadline := persisted.Status.AtProvider.RetiredKeys[0].DeleteAfter
				require.NotNil(t, deadline, "normal Observe must initialize fallback state without metadata drift; rotation=%#v policies=%v", persisted.Spec.ForProvider.Rotation, persisted.Spec.ManagementPolicies)
				require.True(t, deadline.Time.Equal(now.Add(48*time.Hour)))
				require.Equal(t, rotationTestK1, persisted.Status.AtProvider.GUID)
				require.Equal(t, target, persisted.Annotations[servicecredentialbinding.ForceRotationKey], "a deferred or stale target remains inert and is not consumed")

				// Reload and reconcile again to prove the deadline is durable and does not slide.
				_, err = reconciler.Reconcile(context.Background(), request)
				require.NoError(t, err)
				reloaded := &v1alpha1.ServiceCredentialBinding{}
				require.NoError(t, store.Get(context.Background(), types.NamespacedName{Name: cr.Name}, reloaded))
				require.True(t, reloaded.Status.AtProvider.RetiredKeys[0].DeleteAfter.Time.Equal(deadline.Time))
				require.Equal(t, rotationTestK1, reloaded.Status.AtProvider.GUID, "no replacement may be created when Create is disallowed")

				now = deadline.Time
				if tc.wantCleanup {
					cf.On("Get", mock.Anything, rotationTestK0).Return(rotationControllerBinding(rotationTestK0, v1alpha1.LastOperationSucceeded), nil).Once()
					cf.On("Delete", mock.Anything, rotationTestK0).Return("", nil).Once()
					cf.On("Get", mock.Anything, rotationTestK0).Return((*cfresource.ServiceCredentialBinding)(nil), cfresource.NewResourceNotFoundError()).Once()
				}
				_, err = reconciler.Reconcile(context.Background(), request)
				require.NoError(t, err)
				cleaned := &v1alpha1.ServiceCredentialBinding{}
				require.NoError(t, store.Get(context.Background(), types.NamespacedName{Name: cr.Name}, cleaned))
				require.Equal(t, rotationTestK1, cleaned.Status.AtProvider.GUID, "routine cleanup must protect the active generation")
				if tc.wantCleanup {
					require.Empty(t, cleaned.Status.AtProvider.RetiredKeys, "Update-permitted reconciliation should clean at the deadline")
				} else {
					require.Len(t, cleaned.Status.AtProvider.RetiredKeys, 1, "Create-only policy must not gain cleanup authority")
				}
			} else {
				persisted := &v1alpha1.ServiceCredentialBinding{}
				require.NoError(t, store.Get(context.Background(), types.NamespacedName{Name: cr.Name}, persisted))
				require.Nil(t, persisted.Status.AtProvider.RetiredKeys[0].DeleteAfter, "disallowed or paused lifecycle must not initialize fallback state")
			}
			cf.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
			if !tc.wantCleanup {
				cf.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
			}
			cf.AssertExpectations(t)
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

func TestRotationProviderRotationWithoutUpdatePolicy(t *testing.T) {
	cr := rotationHandoverCR(rotationTestK0, rotationTestK0, "key")
	cr.Status.AtProvider.RetiredKeys = nil
	cr.Spec.ManagementPolicies = xpv2.ManagementPolicies{xpv2.ManagementActionObserve, xpv2.ManagementActionCreate}
	cr.Spec.DeletionPolicy = xpv2.DeletionOrphan
	cr.Finalizers = []string{managed.FinalizerName}
	store, scheme := rotationRuntimeStore(t, cr)
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Name: "rotation-credentials", Namespace: "rotation-test",
		OwnerReferences: []metav1.OwnerReference{{APIVersion: v1alpha1.CRDGroupVersion.String(), Kind: v1alpha1.ServiceCredentialBindingKind, Name: cr.Name, UID: cr.UID, Controller: ptr.To(true)}},
	}, Type: resource.SecretTypeConnection, Data: map[string][]byte{"username": []byte("current")}}
	require.NoError(t, store.Create(context.Background(), secret))
	cf := &fake.MockServiceCredentialBinding{}
	cf.On("Get", mock.Anything, rotationTestK0).Return(rotationControllerBinding(rotationTestK0, v1alpha1.LastOperationSucceeded), nil).Once()
	cf.On("GetDetails", mock.Anything, rotationTestK0).Return(&cfresource.ServiceCredentialBindingDetails{Credentials: map[string]any{"username": "current"}}, nil).Once()
	cf.On("Create", mock.Anything, mock.Anything).Return("", rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationSucceeded), nil).Once()
	cf.On("Single", mock.Anything, mock.Anything).Return(rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationSucceeded), nil).Once()
	ext := &external{
		kube: store, reader: store, scbClient: cf,
		keyRotator:        &servicecredentialbinding.SCBKeyRotator{SCBClient: cf},
		maxCreateAttempts: DefaultMaxCreateAttempts, managementPoliciesEnabled: true,
	}
	ext.observationStateHandler = ext
	reconciler := rotationRuntimeReconciler(t, store, scheme, ext, true)

	_, err := reconciler.Reconcile(context.Background(), reconcile.Request{NamespacedName: types.NamespacedName{Name: cr.Name}})
	require.NoError(t, err)
	cf.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	cf.AssertExpectations(t)
	persisted := &v1alpha1.ServiceCredentialBinding{}
	require.NoError(t, store.Get(context.Background(), types.NamespacedName{Name: cr.Name}, persisted))
	require.Len(t, persisted.Status.AtProvider.RetiredKeys, 1)
	require.Equal(t, rotationTestK0, persisted.Status.AtProvider.RetiredKeys[0].GUID)
	require.Equal(t, rotationTestK0, persisted.GetAnnotations()[servicecredentialbinding.ForceRotationKey])
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
	cf.On("GetDetails", mock.Anything, mock.Anything).Return(&cfresource.ServiceCredentialBindingDetails{Credentials: map[string]any{}}, nil).Maybe()
	c := &external{kube: kube, scbClient: cf, keyRotator: &servicecredentialbinding.SCBKeyRotator{SCBClient: cf}, maxCreateAttempts: DefaultMaxCreateAttempts, managementPoliciesEnabled: true}
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

func rotationRuntimeReconciler(t *testing.T, kube client.Client, scheme *runtime.Scheme, external managed.ExternalClient, enablePolicies bool, extra ...managed.ReconcilerOption) *managed.Reconciler {
	t.Helper()
	options := []managed.ReconcilerOption{
		managed.WithInitializers(),
		managed.WithExternalConnector(rotationTestExternalConnector{external: external}),
		managed.WithPollInterval(time.Millisecond),
	}
	if enablePolicies {
		options = append(options, managed.WithManagementPolicies())
	}
	options = append(options, extra...)
	return managed.NewReconciler(&rotationTestManager{kube: kube, scheme: scheme}, resource.ManagedKind(v1alpha1.ServiceCredentialBindingGroupVersionKind), options...)
}

type rotationTestExternalConnector struct{ external managed.ExternalClient }

func (c rotationTestExternalConnector) Connect(context.Context, resource.Managed) (managed.ExternalClient, error) {
	return c.external, nil
}
