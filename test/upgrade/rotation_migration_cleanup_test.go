package upgrade

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
)

const rotationMigrationTestBindingName = "rotation-migration-cleanup-test"

func TestPrepareRotationMigrationBindingDeletionArmsBeforeDelete(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	binding := &v1alpha1.ServiceCredentialBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       rotationMigrationTestBindingName,
			Finalizers: []string{managed.FinalizerName},
			Annotations: map[string]string{
				rotationMigrationForceAnnotation: "",
				rotationMigrationPauseAnnotation: "retry-exhausted",
				"example.test/preserve":          "yes",
			},
		},
		Spec: v1alpha1.ServiceCredentialBindingSpec{
			DeletionPolicy:     xpv2.DeletionDelete,
			ManagementPolicies: xpv2.ManagementPolicies{xpv2.ManagementActionObserve, xpv2.ManagementActionUpdate},
		},
	}
	meta.SetExternalName(binding, "active-guid")
	binding.Status.AtProvider.GUID = "active-guid"
	binding.Status.AtProvider.RetiredKeys = []*v1alpha1.SCBResource{{GUID: "retired-guid"}}
	store := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(binding).WithObjects(binding).Build()
	observed := &rotationMigrationCleanupClient{Client: store}
	manager := &rotationPauseTestManager{kube: observed, scheme: scheme}
	var deleteCalls int
	external := managed.ExternalClientFns{
		ObserveFn: func(context.Context, resource.Managed) (managed.ExternalObservation, error) {
			return managed.ExternalObservation{ResourceExists: true}, nil
		},
		CreateFn: func(context.Context, resource.Managed) (managed.ExternalCreation, error) {
			t.Error("cleanup reconciliation must not create a binding")
			return managed.ExternalCreation{}, nil
		},
		UpdateFn: func(context.Context, resource.Managed) (managed.ExternalUpdate, error) {
			return managed.ExternalUpdate{}, nil
		},
		DeleteFn: func(_ context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
			deleteCalls++
			require.Equal(t, "active-guid", mg.(*v1alpha1.ServiceCredentialBinding).Status.AtProvider.GUID)
			require.Len(t, mg.(*v1alpha1.ServiceCredentialBinding).Status.AtProvider.RetiredKeys, 1)
			return managed.ExternalDelete{}, nil
		},
		DisconnectFn: func(context.Context) error { return nil },
	}
	reconciler := managed.NewReconciler(manager, resource.ManagedKind(v1alpha1.ServiceCredentialBindingGroupVersionKind),
		managed.WithManagementPolicies(),
		managed.WithExternalConnector(managed.ExternalConnectorFn(func(context.Context, resource.Managed) (managed.ExternalClient, error) {
			return external, nil
		})),
	)
	observed.reconcileWhileDeleting = func() {
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: binding.Name}})
		require.NoError(t, err)
		require.Zero(t, deleteCalls, "the full pause must prevent external calls between Delete and release")
		persisted := &v1alpha1.ServiceCredentialBinding{}
		require.NoError(t, store.Get(ctx, client.ObjectKey{Name: binding.Name}, persisted))
		require.NotNil(t, persisted.DeletionTimestamp)
		require.Contains(t, persisted.Finalizers, managed.FinalizerName, "the paused reconciler must not orphan-finalize the binding")
	}

	var remembered []string
	err := prepareRotationMigrationBindingDeletion(ctx, observed, client.ObjectKey{Name: binding.Name}, func(current *v1alpha1.ServiceCredentialBinding) {
		remembered = append(remembered, current.Status.AtProvider.GUID)
		for _, retired := range current.Status.AtProvider.RetiredKeys {
			if retired != nil {
				remembered = append(remembered, retired.GUID)
			}
		}
	})
	require.NoError(t, err)
	require.NotEmpty(t, remembered)
	require.Contains(t, remembered, "active-guid")
	require.Contains(t, remembered, "retired-guid")
	require.Equal(t, 4, observed.updateCalls, "arm and release each retry once after a conflict")
	require.Equal(t, 1, observed.deleteCalls)

	persisted := &v1alpha1.ServiceCredentialBinding{}
	require.NoError(t, store.Get(ctx, client.ObjectKey{Name: binding.Name}, persisted))
	require.Equal(t, xpv2.ManagementPolicies{xpv2.ManagementActionAll}, persisted.Spec.ManagementPolicies)
	require.Equal(t, "yes", persisted.Annotations["example.test/preserve"])
	require.NotContains(t, persisted.Annotations, rotationMigrationForceAnnotation)
	require.NotContains(t, persisted.Annotations, rotationMigrationPauseAnnotation)
	require.NotContains(t, persisted.Annotations, meta.AnnotationKeyReconciliationPaused)
	require.NotNil(t, persisted.DeletionTimestamp)
	require.Equal(t, "active-guid", persisted.Status.AtProvider.GUID)
	require.Len(t, persisted.Status.AtProvider.RetiredKeys, 1)
	require.Equal(t, "retired-guid", persisted.Status.AtProvider.RetiredKeys[0].GUID)

	_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: binding.Name}})
	require.NoError(t, err)
	require.Equal(t, 1, deleteCalls, "after release, effective Delete must reach the external client")
}

type rotationMigrationCleanupClient struct {
	client.Client
	updateCalls            int
	deleteCalls            int
	reconcileWhileDeleting func()
	deleteErr              error
	failUpdateAt           int
	updateErr              error
}

func (c *rotationMigrationCleanupClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	c.updateCalls++
	if c.updateCalls == c.failUpdateAt {
		return c.updateErr
	}
	// Exercise RetryOnConflict with a fresh Get on both the arm and release writes.
	if c.updateCalls == 1 || c.updateCalls == 3 {
		return apierrors.NewConflict(schema.GroupResource{Group: v1alpha1.CRDGroupVersion.Group, Resource: "servicecredentialbindings"}, obj.GetName(), errors.New("injected conflict"))
	}
	return c.Client.Update(ctx, obj, opts...)
}

func (c *rotationMigrationCleanupClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	c.deleteCalls++
	persisted := &v1alpha1.ServiceCredentialBinding{}
	if err := c.Get(ctx, client.ObjectKeyFromObject(obj), persisted); err != nil {
		return err
	}
	if persisted.Annotations[meta.AnnotationKeyReconciliationPaused] != "true" || !slices.Equal(persisted.Spec.ManagementPolicies, xpv2.ManagementPolicies{xpv2.ManagementActionAll}) {
		return errors.New("Delete called before full pause and wildcard policies were persisted together")
	}
	if c.deleteErr != nil {
		return c.deleteErr
	}
	if err := c.Client.Delete(ctx, obj, opts...); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if c.reconcileWhileDeleting != nil {
		c.reconcileWhileDeleting()
	}
	return nil
}

func TestPrepareRotationMigrationBindingDeletionDoesNotReleaseAfterDeleteFailure(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))
	binding := &v1alpha1.ServiceCredentialBinding{ObjectMeta: metav1.ObjectMeta{
		Name: rotationMigrationTestBindingName,
		Annotations: map[string]string{
			rotationMigrationForceAnnotation: "",
			rotationMigrationPauseAnnotation: "retry-exhausted",
		},
	}, Spec: v1alpha1.ServiceCredentialBindingSpec{ManagementPolicies: xpv2.ManagementPolicies{xpv2.ManagementActionObserve, xpv2.ManagementActionUpdate}}}
	store := fake.NewClientBuilder().WithScheme(scheme).WithObjects(binding).Build()
	injected := errors.New("injected Delete failure")
	observed := &rotationMigrationCleanupClient{Client: store, deleteErr: injected}

	err := prepareRotationMigrationBindingDeletion(ctx, observed, client.ObjectKeyFromObject(binding), func(*v1alpha1.ServiceCredentialBinding) {})
	require.ErrorIs(t, err, injected)
	persisted := &v1alpha1.ServiceCredentialBinding{}
	require.NoError(t, store.Get(ctx, client.ObjectKeyFromObject(binding), persisted))
	require.Equal(t, "true", persisted.Annotations[meta.AnnotationKeyReconciliationPaused])
	require.Empty(t, persisted.Annotations[rotationMigrationForceAnnotation])
	require.Equal(t, "retry-exhausted", persisted.Annotations[rotationMigrationPauseAnnotation])
	require.Equal(t, xpv2.ManagementPolicies{xpv2.ManagementActionAll}, persisted.Spec.ManagementPolicies)
}

func TestPrepareRotationMigrationBindingDeletionReportsArmAndReleaseFailures(t *testing.T) {
	for _, tc := range []struct {
		name          string
		failUpdateAt  int
		wantDelete    int
		wantTimestamp bool
	}{
		{name: "arm failure prevents delete", failUpdateAt: 1},
		{name: "release failure leaves binding paused", failUpdateAt: 4, wantDelete: 1, wantTimestamp: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			scheme := runtime.NewScheme()
			require.NoError(t, v1alpha1.AddToScheme(scheme))
			binding := &v1alpha1.ServiceCredentialBinding{ObjectMeta: metav1.ObjectMeta{
				Name:       rotationMigrationTestBindingName,
				Finalizers: []string{managed.FinalizerName},
				Annotations: map[string]string{
					rotationMigrationForceAnnotation: "",
					rotationMigrationPauseAnnotation: "retry-exhausted",
				},
			}}
			store := fake.NewClientBuilder().WithScheme(scheme).WithObjects(binding).Build()
			injected := errors.New("injected Kubernetes update failure")
			observed := &rotationMigrationCleanupClient{Client: store, failUpdateAt: tc.failUpdateAt, updateErr: injected}
			err := prepareRotationMigrationBindingDeletion(ctx, observed, client.ObjectKeyFromObject(binding), func(*v1alpha1.ServiceCredentialBinding) {})
			require.ErrorIs(t, err, injected)
			require.Equal(t, tc.wantDelete, observed.deleteCalls)
			persisted := &v1alpha1.ServiceCredentialBinding{}
			require.NoError(t, store.Get(ctx, client.ObjectKeyFromObject(binding), persisted))
			require.Equal(t, tc.wantTimestamp, persisted.DeletionTimestamp != nil)
			if tc.wantTimestamp {
				require.Equal(t, "true", persisted.Annotations[meta.AnnotationKeyReconciliationPaused])
				require.Contains(t, persisted.Annotations, rotationMigrationForceAnnotation)
			}
		})
	}
}

func TestPrepareRotationMigrationBindingDeletionIsSafeWhenRepeatedAndAlreadyPaused(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))
	binding := &v1alpha1.ServiceCredentialBinding{ObjectMeta: metav1.ObjectMeta{
		Name:       rotationMigrationTestBindingName,
		Finalizers: []string{managed.FinalizerName},
		Annotations: map[string]string{
			meta.AnnotationKeyReconciliationPaused: "true",
			rotationMigrationForceAnnotation:       "",
			"example.test/keep":                    "yes",
		},
	}, Spec: v1alpha1.ServiceCredentialBindingSpec{ManagementPolicies: xpv2.ManagementPolicies{xpv2.ManagementActionObserve, xpv2.ManagementActionUpdate}}}
	store := fake.NewClientBuilder().WithScheme(scheme).WithObjects(binding).Build()
	observed := &rotationMigrationCleanupClient{Client: store}
	key := client.ObjectKeyFromObject(binding)
	remember := func(*v1alpha1.ServiceCredentialBinding) {}
	require.NoError(t, prepareRotationMigrationBindingDeletion(ctx, observed, key, remember))
	require.NoError(t, prepareRotationMigrationBindingDeletion(ctx, observed, key, remember))
	require.Equal(t, 2, observed.deleteCalls)
	persisted := &v1alpha1.ServiceCredentialBinding{}
	require.NoError(t, store.Get(ctx, key, persisted))
	require.Equal(t, "yes", persisted.Annotations["example.test/keep"])
	require.NotContains(t, persisted.Annotations, meta.AnnotationKeyReconciliationPaused)
	require.NotContains(t, persisted.Annotations, rotationMigrationForceAnnotation)
	require.Equal(t, xpv2.ManagementPolicies{xpv2.ManagementActionAll}, persisted.Spec.ManagementPolicies)
}

func TestPrepareRotationMigrationBindingDeletionToleratesMissingAndNilAnnotations(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))
	kube := fake.NewClientBuilder().WithScheme(scheme).Build()
	key := client.ObjectKey{Name: "missing"}
	require.NoError(t, prepareRotationMigrationBindingDeletion(ctx, kube, key, func(*v1alpha1.ServiceCredentialBinding) {}))

	binding := &v1alpha1.ServiceCredentialBinding{ObjectMeta: metav1.ObjectMeta{Name: "no-annotations", Finalizers: []string{managed.FinalizerName}}}
	require.NoError(t, kube.Create(ctx, binding))
	require.NoError(t, prepareRotationMigrationBindingDeletion(ctx, kube, client.ObjectKeyFromObject(binding), func(*v1alpha1.ServiceCredentialBinding) {}))
	persisted := &v1alpha1.ServiceCredentialBinding{}
	require.NoError(t, kube.Get(ctx, client.ObjectKeyFromObject(binding), persisted))
	require.Nil(t, persisted.Annotations)
}
