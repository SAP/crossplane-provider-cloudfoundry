package servicecredentialbinding

import (
	"context"
	"errors"
	"fmt"
	"time"

	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
)

const ForceRotationKey = "servicecredentialbinding.cloudfoundry.crossplane.io/force-rotation"

type KeyRotator interface {
	// RetireBinding checks if the binding should be retired based on the rotation frequency
	// and the force rotation annotation. If it should be retired, it adds the retired key to the status.
	RetireBinding(cr *v1alpha1.ServiceCredentialBinding, serviceBinding *cfresource.ServiceCredentialBinding) bool

	// HasExpiredKeys checks if there are any retired keys that have expired based on the rotation TTL.
	HasExpiredKeys(cr *v1alpha1.ServiceCredentialBinding) bool

	// DeleteExpiredKeys deletes the expired keys from the status and the external system.
	// It returns the new list of retired keys and any error encountered during deletion.
	DeleteExpiredKeys(ctx context.Context, cr *v1alpha1.ServiceCredentialBinding) ([]*v1alpha1.SCBResource, error)

	// DeleteRetiredKeys deletes all retired keys from the external system.
	DeleteRetiredKeys(ctx context.Context, cr *v1alpha1.ServiceCredentialBinding) error
}

type SCBKeyRotator struct {
	SCBClient ServiceCredentialBinding
}

func (r *SCBKeyRotator) RetireBinding(cr *v1alpha1.ServiceCredentialBinding, serviceBinding *cfresource.ServiceCredentialBinding) bool {
	forceRotation := false
	if cr.ObjectMeta.Annotations != nil {
		_, forceRotation = cr.ObjectMeta.Annotations[ForceRotationKey]
	}

	rotationDue := cr.Spec.ForProvider.Rotation != nil && cr.Spec.ForProvider.Rotation.Frequency != nil &&
		(cr.Status.AtProvider.CreatedAt == nil ||
			cr.Status.AtProvider.CreatedAt.Add(cr.Spec.ForProvider.Rotation.Frequency.Duration).Before(time.Now()))

	if forceRotation || rotationDue {
		// If the binding was created before the rotation frequency, retire it.
		for _, retiredKey := range cr.Status.AtProvider.RetiredKeys {
			if retiredKey.GUID == serviceBinding.GUID {
				// If the binding is already retired, do not retire it again.
				return true
			}
		}
		cr.Status.AtProvider.RetiredKeys = append(cr.Status.AtProvider.RetiredKeys, &v1alpha1.SCBResource{
			GUID:      serviceBinding.GUID,
			CreatedAt: &metav1.Time{Time: serviceBinding.CreatedAt},
		})
		return true
	}

	return false
}

func (r *SCBKeyRotator) HasExpiredKeys(cr *v1alpha1.ServiceCredentialBinding) bool {
	if cr.Status.AtProvider.RetiredKeys == nil || cr.Spec.ForProvider.Rotation == nil ||
		cr.Spec.ForProvider.Rotation.TTL == nil {
		return false
	}

	for _, key := range cr.Status.AtProvider.RetiredKeys {
		if key.CreatedAt.Add(cr.Spec.ForProvider.Rotation.TTL.Duration).Before(time.Now()) {
			return true
		}
	}

	return false
}

func (c *SCBKeyRotator) DeleteExpiredKeys(ctx context.Context, cr *v1alpha1.ServiceCredentialBinding) ([]*v1alpha1.SCBResource, error) {
	var newRetiredKeys []*v1alpha1.SCBResource
	var errs []error

	for _, key := range cr.Status.AtProvider.RetiredKeys {

		if key.CreatedAt.Add(cr.Spec.ForProvider.Rotation.TTL.Duration).After(time.Now()) ||
			key.GUID == meta.GetExternalName(cr) {
			newRetiredKeys = append(newRetiredKeys, key)

		} else if err := Delete(ctx, c.SCBClient, key.GUID); err != nil &&
			!cfresource.IsResourceNotFoundError(err) &&
			!cfresource.IsServiceBindingNotFoundError(err) {

			// If we cannot delete the key, keep it in the list
			newRetiredKeys = append(newRetiredKeys, key)
			errs = append(errs, fmt.Errorf("cannot delete expired key %s: %w", key.GUID, err))
		}
	}

	return newRetiredKeys, errors.Join(errs...)
}

func (c *SCBKeyRotator) DeleteRetiredKeys(ctx context.Context, cr *v1alpha1.ServiceCredentialBinding) error {
	for _, retiredKey := range cr.Status.AtProvider.RetiredKeys {
		if err := Delete(ctx, c.SCBClient, retiredKey.GUID); err != nil &&
			!cfresource.IsResourceNotFoundError(err) &&
			!cfresource.IsServiceBindingNotFoundError(err) {
			return fmt.Errorf("cannot delete retired key %s: %w", retiredKey.GUID, err)
		}
	}
	return nil
}
