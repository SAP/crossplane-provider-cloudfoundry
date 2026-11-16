package servicecredentialbinding

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/v2/pkg/event"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	scb "github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/servicecredentialbinding"
)

const (
	reasonKeyRotated                  event.Reason = "KeyRotated"
	reasonConnectionSecretRepublished event.Reason = "ConnectionSecretRepublished"
	reasonRetiredKeyDeleted           event.Reason = "RetiredKeyDeleted"
	reasonCannotDeleteRetiredKey      event.Reason = "CannotDeleteRetiredKey"
)

func (c *external) emit(cr *v1alpha1.ServiceCredentialBinding, e event.Event) {
	if c.recorder != nil {
		c.recorder.Event(cr, e)
	}
}

// completeRotation emits KeyRotated once for each retired key whose replacement
// is now observed. ReplacementGUID is persisted first so that later observations
// do not repeat the event.
func (c *external) completeRotation(ctx context.Context, cr *v1alpha1.ServiceCredentialBinding, active string) error {
	if !isKeyBinding(cr) || active == "" {
		return nil
	}
	for i := range cr.Status.AtProvider.RetiredKeys {
		retired := cr.Status.AtProvider.RetiredKeys[i]
		if retired == nil || retired.GUID == "" || retired.RotationTrigger == "" || retired.ReplacementGUID != "" || scb.SameGUID(retired.GUID, active) {
			continue
		}
		retired.ReplacementGUID = active
		if err := c.kube.Status().Update(ctx, cr); err != nil {
			// The runtime writes this object back after the error; undo so the
			// event is retried.
			retired.ReplacementGUID = ""
			return fmt.Errorf("cannot persist completed key rotation: %w", err)
		}
		c.emit(cr, event.Normal(reasonKeyRotated, fmt.Sprintf("Rotated key %s to %s (%s)", retired.GUID, active, retired.RotationTrigger)))
	}
	return nil
}

// recordSecretRepublish reports that the runtime is about to overwrite the
// controlled connection Secret with the active key's credentials. A failed write
// is reported by the runtime's own CannotPublishConnectionDetails warning.
func (c *external) recordSecretRepublish(cr *v1alpha1.ServiceCredentialBinding, active string) {
	ref := cr.Spec.WriteConnectionSecretToReference
	c.emit(cr, event.Normal(reasonConnectionSecretRepublished, fmt.Sprintf("Connection Secret %s/%s lacks the credentials of key %s; republishing it", ref.Namespace, ref.Name, active)))
}

// recordDeletedKeys emits one event per retired key that routine cleanup removed
// from tracking. Call it only after the shorter list has been persisted.
func (c *external) recordDeletedKeys(cr *v1alpha1.ServiceCredentialBinding, before, after []*v1alpha1.SCBResource) {
	kept := make(map[string]bool, len(after))
	for _, key := range after {
		if key != nil {
			kept[key.GUID] = true
		}
	}
	for _, key := range before {
		if key != nil && !kept[key.GUID] {
			c.emit(cr, event.Normal(reasonRetiredKeyDeleted, fmt.Sprintf("Deleted retired key %s from Cloud Foundry after its retention period", key.GUID)))
		}
	}
}
