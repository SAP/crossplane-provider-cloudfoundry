package upgrade

import (
	"context"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
)

func TestResumePausedRotationMigrationParents(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	org := &v1alpha1.Organization{}
	org.Name = "rotation-org"
	org.Annotations = map[string]string{meta.AnnotationKeyReconciliationPaused: "true", "example.test/keep": "yes"}
	space := &v1alpha1.Space{}
	space.Name = "rotation-space"
	space.Annotations = map[string]string{meta.AnnotationKeyReconciliationPaused: "true"}
	instance := &v1alpha1.ServiceInstance{}
	instance.Name = "rotation-instance"
	instance.Annotations = map[string]string{meta.AnnotationKeyReconciliationPaused: "true"}

	kube := fake.NewClientBuilder().WithScheme(scheme).WithObjects(org, space, instance).Build()
	err := resumePausedFixtureResources(context.Background(), kube,
		&v1alpha1.Organization{ObjectMeta: org.ObjectMeta},
		&v1alpha1.Space{ObjectMeta: space.ObjectMeta},
		&v1alpha1.ServiceInstance{ObjectMeta: instance.ObjectMeta},
	)
	require.NoError(t, err)

	for _, fixture := range []struct {
		name string
		obj  client.Object
	}{
		{name: org.Name, obj: &v1alpha1.Organization{}},
		{name: space.Name, obj: &v1alpha1.Space{}},
		{name: instance.Name, obj: &v1alpha1.ServiceInstance{}},
	} {
		require.NoError(t, kube.Get(context.Background(), client.ObjectKey{Name: fixture.name}, fixture.obj))
		require.Equal(t, "false", fixture.obj.GetAnnotations()[meta.AnnotationKeyReconciliationPaused], fixture.name)
	}
	persistedOrg := &v1alpha1.Organization{}
	require.NoError(t, kube.Get(context.Background(), client.ObjectKey{Name: org.Name}, persistedOrg))
	require.Equal(t, "yes", persistedOrg.Annotations["example.test/keep"], "resume must preserve unrelated annotations")
}

func TestResumePausedRotationMigrationParentsAllowsPausedFinalizersToComplete(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))
	objects := []resource.Managed{
		&v1alpha1.Organization{},
		&v1alpha1.Space{},
		&v1alpha1.ServiceInstance{},
	}
	kinds := []schema.GroupVersionKind{
		v1alpha1.Org_GroupVersionKind,
		v1alpha1.Space_GroupVersionKind,
		v1alpha1.ServiceInstance_GroupVersionKind,
	}
	prototypes := make([]client.Object, 0, len(objects))
	for i, obj := range objects {
		obj.SetName("paused-fixture-" + kinds[i].Kind)
		obj.SetAnnotations(map[string]string{meta.AnnotationKeyReconciliationPaused: "true"})
		policies := xpv2.ManagementPolicies{xpv2.ManagementActionObserve}
		switch typed := obj.(type) {
		case *v1alpha1.Organization:
			typed.Spec.ManagementPolicies = policies
			typed.Spec.DeletionPolicy = xpv2.DeletionOrphan
		case *v1alpha1.Space:
			typed.Spec.ManagementPolicies = policies
			typed.Spec.DeletionPolicy = xpv2.DeletionOrphan
		case *v1alpha1.ServiceInstance:
			typed.Spec.ManagementPolicies = policies
			typed.Spec.DeletionPolicy = xpv2.DeletionOrphan
		}
		obj.SetFinalizers([]string{managed.FinalizerName})
		prototypes = append(prototypes, obj.(client.Object))
	}
	kube := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(prototypes...).WithObjects(prototypes...).Build()
	manager := &rotationPauseTestManager{kube: kube, scheme: scheme}

	for i, obj := range objects {
		kind := kinds[i]
		reconciler := managed.NewReconciler(manager, resource.ManagedKind(kind), managed.WithManagementPolicies())
		request := reconcile.Request{NamespacedName: types.NamespacedName{Name: obj.GetName()}}
		require.NoError(t, kube.Delete(context.Background(), obj))
		_, err := reconciler.Reconcile(context.Background(), request)
		require.NoError(t, err)
		remaining := obj.DeepCopyObject().(client.Object)
		require.NoError(t, kube.Get(context.Background(), request.NamespacedName, remaining))
		require.Equal(t, "ReconcilePaused", string(remaining.(resource.Managed).GetCondition(xpv2.TypeSynced).Reason))
		require.Contains(t, remaining.GetFinalizers(), managed.FinalizerName)
	}

	require.NoError(t, resumePausedFixtureResources(context.Background(), kube, prototypes...))
	for i, obj := range objects {
		kind := kinds[i]
		reconciler := managed.NewReconciler(manager, resource.ManagedKind(kind), managed.WithManagementPolicies())
		request := reconcile.Request{NamespacedName: types.NamespacedName{Name: obj.GetName()}}
		_, err := reconciler.Reconcile(context.Background(), request)
		require.NoError(t, err)
		remaining := obj.DeepCopyObject().(client.Object)
		err = kube.Get(context.Background(), request.NamespacedName, remaining)
		require.True(t, apierrors.IsNotFound(err), "%s finalizer should complete after resume, got %v", kind.Kind, err)
	}
}

type rotationPauseTestManager struct {
	manager.Manager
	kube   client.Client
	scheme *runtime.Scheme
}

func (m *rotationPauseTestManager) GetClient() client.Client   { return m.kube }
func (m *rotationPauseTestManager) GetScheme() *runtime.Scheme { return m.scheme }

func TestResumePausedRotationMigrationParentsToleratesPartialSetup(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))
	org := &v1alpha1.Organization{}
	org.Name = "rotation-org"
	org.Annotations = map[string]string{meta.AnnotationKeyReconciliationPaused: "true"}
	kube := fake.NewClientBuilder().WithScheme(scheme).WithObjects(org).Build()

	err := resumePausedFixtureResources(context.Background(), kube,
		&v1alpha1.Organization{ObjectMeta: org.ObjectMeta},
		&v1alpha1.Space{ObjectMeta: metav1.ObjectMeta{Name: "not-created"}},
	)
	require.NoError(t, err)
	persisted := &v1alpha1.Organization{}
	require.NoError(t, kube.Get(context.Background(), client.ObjectKey{Name: org.Name}, persisted))
	require.Equal(t, "false", persisted.Annotations[meta.AnnotationKeyReconciliationPaused])
}
