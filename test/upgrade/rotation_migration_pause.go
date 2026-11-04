package upgrade

import (
	"context"
	"errors"
	"fmt"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// resumePausedFixtureResources resumes only the explicitly supplied upgrade
// fixtures. It is safe after partial setup and can be called again during
// teardown if post-upgrade verification failed before the normal resume point.
func resumePausedFixtureResources(ctx context.Context, kube client.Client, fixtures ...client.Object) error {
	var errs []error
	for _, fixture := range fixtures {
		key := client.ObjectKeyFromObject(fixture)
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			current := fixture.DeepCopyObject().(client.Object)
			if err := kube.Get(ctx, key, current); err != nil {
				if apierrors.IsNotFound(err) {
					return nil
				}
				return err
			}

			annotations := current.GetAnnotations()
			if annotations[meta.AnnotationKeyReconciliationPaused] != "true" {
				return nil
			}
			annotations[meta.AnnotationKeyReconciliationPaused] = "false"
			current.SetAnnotations(annotations)
			return kube.Update(ctx, current)
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("resume fixture %q: %w", key.Name, err))
		}
	}
	return errors.Join(errs...)
}
