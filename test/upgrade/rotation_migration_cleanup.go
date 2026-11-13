package upgrade

import (
	"context"
	"fmt"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	rotationMigrationForceAnnotation = "servicecredentialbinding.cloudfoundry.crossplane.io/force-rotation"
	rotationMigrationPauseAnnotation = "servicecredentialbinding.cloudfoundry.crossplane.io/max-retry-exceeded"
)

// prepareRotationMigrationBindingDeletion pauses the managed reconciler and
// grants its deletion policy in one persisted update before requesting deletion.
// It releases only scenario-owned operational annotations after Delete succeeds
// or reports that the binding is already absent.
func prepareRotationMigrationBindingDeletion(
	ctx context.Context,
	kube client.Client,
	key client.ObjectKey,
	remember func(*v1alpha1.ServiceCredentialBinding),
) error {
	var armed *v1alpha1.ServiceCredentialBinding
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current := &v1alpha1.ServiceCredentialBinding{}
		if err := kube.Get(ctx, key, current); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		remember(current)
		if current.Annotations == nil {
			current.Annotations = make(map[string]string)
		}
		current.Spec.ManagementPolicies = xpv2.ManagementPolicies{xpv2.ManagementActionAll}
		current.Annotations[meta.AnnotationKeyReconciliationPaused] = "true"
		if err := kube.Update(ctx, current); err != nil {
			return err
		}
		armed = current
		return nil
	})
	if err != nil {
		return fmt.Errorf("arm owned binding for safe deletion: %w", err)
	}
	if armed == nil {
		return nil
	}

	if err := kube.Delete(ctx, armed); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("request deletion of safely armed binding: %w", err)
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current := &v1alpha1.ServiceCredentialBinding{}
		if err := kube.Get(ctx, key, current); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		remember(current)
		current.Spec.ManagementPolicies = xpv2.ManagementPolicies{xpv2.ManagementActionAll}
		for _, annotation := range []string{
			rotationMigrationForceAnnotation,
			rotationMigrationPauseAnnotation,
			meta.AnnotationKeyReconciliationPaused,
		} {
			delete(current.Annotations, annotation)
		}
		if len(current.Annotations) == 0 {
			current.Annotations = nil
		}
		if err := kube.Update(ctx, current); err != nil {
			return fmt.Errorf("release owned binding after delete request: %w", err)
		}
		return nil
	})
}
