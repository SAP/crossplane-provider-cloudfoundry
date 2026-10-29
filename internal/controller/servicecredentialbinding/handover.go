package servicecredentialbinding

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	scb "github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/servicecredentialbinding"
	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
)

const (
	reasonInvalidForceRotationTarget event.Reason = "InvalidForceRotationTarget"
	reasonForceRotationUnsupported   event.Reason = "ForceRotationUnsupported"
)

func isKeyBinding(cr *v1alpha1.ServiceCredentialBinding) bool {
	return cr != nil && cr.Spec.ForProvider.Type == "key"
}

func isPendingReplacement(cr *v1alpha1.ServiceCredentialBinding, activeGUID string) bool {
	if cr == nil || activeGUID == "" {
		return false
	}
	for _, retired := range cr.Status.AtProvider.RetiredKeys {
		if retired != nil && sameExternalGUID(retired.GUID, activeGUID) {
			return true
		}
	}
	return false
}

func isCanonicalGUID(value string) bool {
	if len(value) != 36 || strings.TrimSpace(value) != value || strings.Count(value, "-") != 4 {
		return false
	}
	_, err := uuid.Parse(value)
	return err == nil
}

func sameExternalGUID(a, b string) bool {
	return a == b || isCanonicalGUID(a) && isCanonicalGUID(b) && strings.EqualFold(a, b)
}

// lifecyclePolicies returns the same effective legacy/management-policy view used by
// Crossplane runtime, including its feature-flag and legacy deletion-policy behavior.
func (c *external) lifecyclePolicies(cr *v1alpha1.ServiceCredentialBinding) (managed.ManagementPoliciesChecker, error) {
	p := managed.NewLegacyManagementPoliciesResolver(c.managementPoliciesEnabled, cr.Spec.ManagementPolicies, cr.Spec.DeletionPolicy)
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return p, nil
}

func (c *external) warnForceRotation(cr *v1alpha1.ServiceCredentialBinding) {
	value, present := cr.GetAnnotations()[scb.ForceRotationKey]
	if !present || c.recorder == nil {
		return
	}
	if cr.Spec.ForProvider.Type == "app" {
		c.recorder.Event(cr, event.Warning(reasonForceRotationUnsupported, errors.New("force rotation is supported only for key bindings")))
		return
	}
	if value == "" || !isCanonicalGUID(value) {
		c.recorder.Event(cr, event.Warning(reasonInvalidForceRotationTarget, errors.New("force-rotation must contain the active key GUID; empty and malformed values are ignored")))
	}
}

func (c *external) now() time.Time {
	if c.nowFn != nil {
		return c.nowFn().UTC()
	}
	if r, ok := c.keyRotator.(*scb.SCBKeyRotator); ok && r.Now != nil {
		return r.Now().UTC()
	}
	return time.Now().UTC()
}

// connectionDetails returns the current CF credentials. Retrieval errors are fatal
// when a connection Secret is configured, because nil details must not masquerade as
// a successful handover/publication.
func (c *external) connectionDetails(ctx context.Context, cr *v1alpha1.ServiceCredentialBinding, binding *cfresource.ServiceCredentialBinding) (managed.ConnectionDetails, error) {
	details, err := scb.GetConnectionDetails(ctx, c.scbClient, binding.GUID, cr.Spec.ConnectionDetailsAsJSON)
	if err != nil && cr.Spec.WriteConnectionSecretToReference != nil {
		return nil, fmt.Errorf("cannot retrieve required connection details: %w", err)
	}
	if err != nil {
		return nil, nil
	}
	return details, nil
}

// confirmHandover requires successful CF operation and, when configured, a
// connection Secret owned by this resource whose complete data matches the freshly
// retrieved details. A missing/unreadable Secret is not proof of publication.
func (c *external) confirmHandover(ctx context.Context, cr *v1alpha1.ServiceCredentialBinding, binding *cfresource.ServiceCredentialBinding, details managed.ConnectionDetails) bool {
	if cr == nil || binding == nil || binding.LastOperation.State != v1alpha1.LastOperationSucceeded {
		return false
	}
	ref := cr.Spec.WriteConnectionSecretToReference
	if ref == nil {
		return true
	}
	if details == nil {
		return false
	}
	reader := c.reader
	if reader == nil {
		reader = c.kube
	}
	if reader == nil {
		return false
	}
	secret := &corev1.Secret{}
	if err := reader.Get(ctx, types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}, secret); err != nil {
		return false
	}
	expected := resource.ConnectionSecretFor(cr, v1alpha1.ServiceCredentialBindingGroupVersionKind)
	if secret.Namespace != expected.Namespace || secret.Name != expected.Name || secret.Type != resource.SecretTypeConnection || !sameControllerOwner(secret, expected) {
		return false
	}
	return connectionDataEqual(secret.Data, details)
}

func sameControllerOwner(actual, expected *corev1.Secret) bool {
	if len(expected.OwnerReferences) != 1 || len(actual.OwnerReferences) != 1 {
		return false
	}
	a, e := actual.OwnerReferences[0], expected.OwnerReferences[0]
	return a.APIVersion == e.APIVersion && a.Kind == e.Kind && a.Name == e.Name && a.UID == e.UID && a.Controller != nil && *a.Controller && e.Controller != nil && *e.Controller
}

func (c *external) preservedConnectionDetails(ctx context.Context, cr *v1alpha1.ServiceCredentialBinding) (managed.ConnectionDetails, error) {
	ref := cr.Spec.WriteConnectionSecretToReference
	if ref == nil {
		return nil, nil
	}
	reader := c.reader
	if reader == nil {
		reader = c.kube
	}
	if reader == nil {
		return nil, errors.New("cannot read configured connection Secret to preserve published credentials")
	}
	secret := &corev1.Secret{}
	if err := reader.Get(ctx, types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}, secret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot read configured connection Secret to preserve published credentials: %w", err)
	}
	expected := resource.ConnectionSecretFor(cr, v1alpha1.ServiceCredentialBindingGroupVersionKind)
	if !sameControllerOwner(secret, expected) {
		return nil, nil
	}
	preserved := make(managed.ConnectionDetails, len(secret.Data))
	for key, value := range secret.Data {
		preserved[key] = append([]byte(nil), value...)
	}
	return preserved, nil
}

func connectionDataEqual(data map[string][]byte, details managed.ConnectionDetails) bool {
	if len(data) != len(details) {
		return false
	}
	for key, value := range details {
		if string(data[key]) != string(value) {
			return false
		}
	}
	return true
}

func (c *external) currentHandover(ctx context.Context, cr *v1alpha1.ServiceCredentialBinding) (*cfresource.ServiceCredentialBinding, managed.ConnectionDetails, bool, error) {
	guid := meta.GetExternalName(cr)
	if !isValidUUID(guid) {
		return nil, nil, false, nil
	}
	binding, err := c.scbClient.Get(ctx, guid)
	if err != nil {
		return nil, nil, false, fmt.Errorf("cannot verify active binding handover: %w", err)
	}
	if cr.Spec.WriteConnectionSecretToReference == nil {
		return binding, nil, c.confirmHandover(ctx, cr, binding, nil), nil
	}
	details, err := c.connectionDetails(ctx, cr, binding)
	if err != nil {
		return binding, nil, false, err
	}
	return binding, details, c.confirmHandover(ctx, cr, binding, details), nil
}

func (c *external) persistFallbackDeadlines(ctx context.Context, cr *v1alpha1.ServiceCredentialBinding, handover time.Time) (bool, error) {
	rotator, ok := c.keyRotator.(*scb.SCBKeyRotator)
	if !ok || !rotator.InitializeFallbackDeadlines(cr, handover) {
		return false, nil
	}
	if err := c.kube.Status().Update(ctx, cr); err != nil {
		return false, fmt.Errorf("cannot persist retired-key retention deadlines: %w", err)
	}
	return true, nil
}
