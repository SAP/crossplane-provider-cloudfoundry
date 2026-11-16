package servicecredentialbinding

import (
	"testing"
	"time"

	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
)

const (
	rotationTestK0 = "11111111-1111-4111-8111-111111111111"
	rotationTestK1 = "22222222-2222-4222-8222-222222222222"
	rotationTestK2 = "33333333-3333-4333-8333-333333333333"
	rotationTestK3 = "44444444-4444-4444-8444-444444444444"
)

func rotationTestBinding(guid string) *cfresource.ServiceCredentialBinding {
	return &cfresource.ServiceCredentialBinding{
		Resource: cfresource.Resource{
			GUID:      guid,
			CreatedAt: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
		Name: ptr.To("service-key-suffix"),
	}
}

func rotationTestCR(bindingType string) *v1alpha1.ServiceCredentialBinding {
	return &v1alpha1.ServiceCredentialBinding{
		ObjectMeta: metav1.ObjectMeta{},
		Spec: v1alpha1.ServiceCredentialBindingSpec{
			ForProvider: v1alpha1.ServiceCredentialBindingParameters{Type: bindingType},
		},
	}
}

func TestSCBKeyRotator_TargetedForceRotation(t *testing.T) {
	tests := []struct {
		name             string
		target           string
		hasAnnotation    bool
		emptyAnnotations bool
		statusGUID       string
		binding          string
		kind             string
		wantRetire       bool
	}{
		{name: "absent annotation", binding: rotationTestK0, kind: "key"},
		{name: "empty annotation map", emptyAnnotations: true, binding: rotationTestK0, kind: "key"},
		{name: "empty annotation", target: "", hasAnnotation: true, binding: rotationTestK0, kind: "key"},
		{name: "matching active GUID", target: rotationTestK0, hasAnnotation: true, binding: rotationTestK0, kind: "key", wantRetire: true},
		{name: "boolean is malformed", target: "true", hasAnnotation: true, binding: rotationTestK0, kind: "key"},
		{name: "timestamp is malformed", target: "2026-10-28T10:00:00Z", hasAnnotation: true, binding: rotationTestK0, kind: "key"},
		{name: "valid unrelated GUID", target: rotationTestK3, hasAnnotation: true, binding: rotationTestK0, kind: "key"},
		{name: "stale GUID does not match observed active binding", target: rotationTestK0, hasAnnotation: true, statusGUID: rotationTestK0, binding: rotationTestK1, kind: "key"},
		{name: "app annotation ignored", target: rotationTestK0, hasAnnotation: true, binding: rotationTestK0, kind: "app"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cr := rotationTestCR(tt.kind)
			if tt.hasAnnotation || tt.emptyAnnotations {
				cr.Annotations = map[string]string{}
			}
			if tt.hasAnnotation {
				cr.Annotations[ForceRotationKey] = tt.target
			}
			cr.Status.AtProvider.GUID = tt.statusGUID
			rotator := &SCBKeyRotator{}
			retired := rotator.RetireBinding(cr, rotationTestBinding(tt.binding))

			require.Equal(t, tt.wantRetire, retired)
			if tt.wantRetire {
				require.Len(t, cr.Status.AtProvider.RetiredKeys, 1)
				got := cr.Status.AtProvider.RetiredKeys[0]
				require.Equal(t, tt.binding, got.GUID)
				require.Equal(t, "service-key-suffix", got.Name)
				require.NotNil(t, got.CreatedAt)
				require.Equal(t, rotationTestBinding(tt.binding).CreatedAt, got.CreatedAt.Time)
			} else {
				require.Empty(t, cr.Status.AtProvider.RetiredKeys)
			}
		})
	}
}

func TestSCBKeyRotator_StaleTargetReplay(t *testing.T) {
	rotator := &SCBKeyRotator{}
	cr := rotationTestCR("key")
	cr.Annotations = map[string]string{ForceRotationKey: rotationTestK0}

	require.True(t, rotator.RetireBinding(cr, rotationTestBinding(rotationTestK0)))
	require.Len(t, cr.Status.AtProvider.RetiredKeys, 1)

	// Replaying K0 after replacement must not retire K1, even while K0 is tracked.
	require.False(t, rotator.RetireBinding(cr, rotationTestBinding(rotationTestK1)))
	require.Len(t, cr.Status.AtProvider.RetiredKeys, 1)
	require.Equal(t, rotationTestK0, cr.Status.AtProvider.RetiredKeys[0].GUID)

	// Once K0 has been garbage-collected, a stale K0/K1 request still cannot
	// retire K2. A request explicitly targeting K2 remains a new operation.
	cr.Status.AtProvider.RetiredKeys = nil
	cr.Annotations[ForceRotationKey] = rotationTestK0
	require.False(t, rotator.RetireBinding(cr, rotationTestBinding(rotationTestK2)))
	require.Empty(t, cr.Status.AtProvider.RetiredKeys)

	cr.Annotations[ForceRotationKey] = rotationTestK1
	require.False(t, rotator.RetireBinding(cr, rotationTestBinding(rotationTestK2)))
	require.Empty(t, cr.Status.AtProvider.RetiredKeys)

	cr.Annotations[ForceRotationKey] = rotationTestK2
	require.True(t, rotator.RetireBinding(cr, rotationTestBinding(rotationTestK2)))
	require.Equal(t, []string{rotationTestK2}, []string{cr.Status.AtProvider.RetiredKeys[0].GUID})
}

func TestSCBKeyRotator_ScheduledRotationIgnoresStaleForceTarget(t *testing.T) {
	cr := rotationTestCR("key")
	cr.Spec.ForProvider.Rotation = &v1alpha1.RotationParameters{
		Frequency: &metav1.Duration{Duration: time.Hour},
	}
	cr.Status.AtProvider.CreatedAt = &metav1.Time{Time: time.Now().Add(-24 * time.Hour)}

	require.True(t, (&SCBKeyRotator{}).RetireBinding(cr, rotationTestBinding(rotationTestK1)))
	require.Len(t, cr.Status.AtProvider.RetiredKeys, 1)
	require.Equal(t, rotationTestK1, cr.Status.AtProvider.RetiredKeys[0].GUID)
}
