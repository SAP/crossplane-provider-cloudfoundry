package servicecredentialbinding

import (
	"context"
	"errors"
	"fmt"
	"time"

	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
)

const (
	ForceRotationKey           = "servicecredentialbinding.cloudfoundry.crossplane.io/force-rotation"
	DefaultRetiredKeyRetention = 48 * time.Hour
)

type KeyRotator interface {
	// RetireBinding checks if the key should be retired based on schedule or target GUID.
	// An already-retired active identity is returned as pending replacement work.
	RetireBinding(cr *v1alpha1.ServiceCredentialBinding, serviceBinding *cfresource.ServiceCredentialBinding) bool

	// HasExpiredKeys checks if a retired key is eligible for routine cleanup.
	HasExpiredKeys(cr *v1alpha1.ServiceCredentialBinding) bool

	// DeleteExpiredKeys deletes eligible retired keys and returns identities still tracked.
	DeleteExpiredKeys(ctx context.Context, cr *v1alpha1.ServiceCredentialBinding) ([]*v1alpha1.SCBResource, error)

	// DeleteRetiredKeys deletes all retired keys as part of full resource deletion.
	DeleteRetiredKeys(ctx context.Context, cr *v1alpha1.ServiceCredentialBinding) error
}

type SCBKeyRotator struct {
	SCBClient ServiceCredentialBinding
	// Now allows lifecycle boundaries to be tested deterministically. A nil value uses UTC wall time.
	Now func() time.Time
}

func (r *SCBKeyRotator) now() time.Time {
	if r.Now != nil {
		return r.Now().UTC()
	}
	return time.Now().UTC()
}

func (r *SCBKeyRotator) RetireBinding(cr *v1alpha1.ServiceCredentialBinding, serviceBinding *cfresource.ServiceCredentialBinding) bool {
	if !eligibleForRetirement(cr, serviceBinding) {
		return false
	}
	if hasPendingRetirement(cr, serviceBinding.GUID) {
		// Resume an accepted replacement independently of the annotation/schedule.
		return true
	}
	if !forceRotationTargetsActiveKey(cr, serviceBinding.GUID) && !r.rotationIsDue(cr) {
		return false
	}

	cr.Status.AtProvider.RetiredKeys = append(cr.Status.AtProvider.RetiredKeys, &v1alpha1.SCBResource{
		GUID:      serviceBinding.GUID,
		Name:      ptr.Deref(serviceBinding.Name, ""),
		CreatedAt: &metav1.Time{Time: serviceBinding.CreatedAt},
	})
	return true
}

func eligibleForRetirement(cr *v1alpha1.ServiceCredentialBinding, binding *cfresource.ServiceCredentialBinding) bool {
	return cr != nil && binding != nil && binding.GUID != "" && cr.Spec.ForProvider.Type != "app" && cr.DeletionTimestamp == nil
}

func hasPendingRetirement(cr *v1alpha1.ServiceCredentialBinding, guid string) bool {
	for _, retiredKey := range cr.Status.AtProvider.RetiredKeys {
		if retiredKey != nil && SameGUID(retiredKey.GUID, guid) {
			return true
		}
	}
	return false
}

func forceRotationTargetsActiveKey(cr *v1alpha1.ServiceCredentialBinding, guid string) bool {
	if cr.Annotations == nil {
		return false
	}
	target, ok := cr.Annotations[ForceRotationKey]
	return ok && IsCanonicalGUID(target) && SameGUID(target, guid)
}

func (r *SCBKeyRotator) rotationIsDue(cr *v1alpha1.ServiceCredentialBinding) bool {
	rotation := cr.Spec.ForProvider.Rotation
	if rotation == nil || rotation.Frequency == nil {
		return false
	}
	createdAt := cr.Status.AtProvider.CreatedAt
	return createdAt == nil || !r.now().Before(createdAt.Add(rotation.Frequency.Duration))
}

// InitializeFallbackDeadlines records a one-time 48-hour fallback deadline for valid,
// non-active retirees. The caller must invoke this only after it has confirmed handover
// and persist the mutation before relying on it for cleanup.
func (r *SCBKeyRotator) InitializeFallbackDeadlines(cr *v1alpha1.ServiceCredentialBinding, handover time.Time) bool {
	if cr == nil || cr.Spec.ForProvider.Rotation != nil && cr.Spec.ForProvider.Rotation.TTL != nil {
		return false
	}

	active := activeGUIDs(cr)
	deadline := handover.UTC().Add(DefaultRetiredKeyRetention)
	// metav1.Time JSON is second precision; round up so serialization cannot shorten retention.
	deadline = deadline.Truncate(time.Second)
	if !deadline.Equal(handover.UTC().Add(DefaultRetiredKeyRetention)) {
		deadline = deadline.Add(time.Second)
	}

	changed := false
	for _, key := range cr.Status.AtProvider.RetiredKeys {
		if key == nil || key.GUID == "" || isActiveGUID(key.GUID, active) || key.DeleteAfter != nil {
			continue
		}
		copy := metav1.NewTime(deadline)
		key.DeleteAfter = &copy
		changed = true
	}
	return changed
}

func (r *SCBKeyRotator) HasExpiredKeys(cr *v1alpha1.ServiceCredentialBinding) bool {
	for _, key := range cr.Status.AtProvider.RetiredKeys {
		if r.expired(cr, key, r.now()) {
			return true
		}
	}
	return false
}

func (r *SCBKeyRotator) DeleteExpiredKeys(ctx context.Context, cr *v1alpha1.ServiceCredentialBinding) ([]*v1alpha1.SCBResource, error) {
	newRetiredKeys := make([]*v1alpha1.SCBResource, 0, len(cr.Status.AtProvider.RetiredKeys))
	errs := make([]error, 0, len(cr.Status.AtProvider.RetiredKeys))
	now := r.now()

	for _, key := range cr.Status.AtProvider.RetiredKeys {
		if key == nil || key.GUID == "" || !r.expired(cr, key, now) {
			newRetiredKeys = append(newRetiredKeys, key)
			continue
		}

		if err := Delete(ctx, r.SCBClient, key.GUID); err != nil {
			newRetiredKeys = append(newRetiredKeys, key)
			errs = append(errs, fmt.Errorf("cannot delete expired key %s: %w", key.GUID, err))
		}
	}

	return newRetiredKeys, errors.Join(errs...)
}

func (r *SCBKeyRotator) expired(cr *v1alpha1.ServiceCredentialBinding, key *v1alpha1.SCBResource, now time.Time) bool {
	if key == nil || key.GUID == "" || isActiveGUID(key.GUID, activeGUIDs(cr)) {
		return false
	}

	var deadline time.Time
	if cr.Spec.ForProvider.Rotation != nil && cr.Spec.ForProvider.Rotation.TTL != nil {
		if key.CreatedAt == nil || key.CreatedAt.IsZero() {
			return false
		}
		deadline = key.CreatedAt.Add(cr.Spec.ForProvider.Rotation.TTL.Duration)
	} else {
		if key.DeleteAfter == nil || key.DeleteAfter.IsZero() {
			return false
		}
		deadline = key.DeleteAfter.Time
	}
	return !now.Before(deadline)
}

func activeGUIDs(cr *v1alpha1.ServiceCredentialBinding) []string {
	active := []string{meta.GetExternalName(cr), cr.Status.AtProvider.GUID}
	return active
}

func isActiveGUID(guid string, active []string) bool {
	for _, candidate := range active {
		if candidate != "" && SameGUID(guid, candidate) {
			return true
		}
	}
	return false
}

func (r *SCBKeyRotator) DeleteRetiredKeys(ctx context.Context, cr *v1alpha1.ServiceCredentialBinding) error {
	for _, retiredKey := range cr.Status.AtProvider.RetiredKeys {
		if retiredKey == nil {
			continue
		}
		if retiredKey.GUID == "" {
			return errors.New("cannot delete retired key with missing GUID")
		}
		if err := Delete(ctx, r.SCBClient, retiredKey.GUID); err != nil {
			return fmt.Errorf("cannot delete retired key %s: %w", retiredKey.GUID, err)
		}
	}
	return nil
}
