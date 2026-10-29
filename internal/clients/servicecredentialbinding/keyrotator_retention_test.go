package servicecredentialbinding

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/fake"
)

func TestRotationFallbackRetention(t *testing.T) {
	now := time.Date(2026, time.October, 28, 9, 0, 0, 0, time.UTC)
	retiree := &v1alpha1.SCBResource{GUID: rotationTestK0, CreatedAt: ptr.To(metav1.NewTime(now.Add(-180 * 24 * time.Hour)))}
	cr := rotationTestCR("key")
	cr.Annotations = map[string]string{"crossplane.io/external-name": rotationTestK1}
	cr.Status.AtProvider.RetiredKeys = []*v1alpha1.SCBResource{retiree}
	rotator := &SCBKeyRotator{Now: func() time.Time { return now }}

	handover := now.Add(30*time.Minute + 250*time.Millisecond)
	require.True(t, rotator.InitializeFallbackDeadlines(cr, handover))
	wantDeadline := handover.Add(DefaultRetiredKeyRetention).Truncate(time.Second).Add(time.Second)
	require.Equal(t, wantDeadline, retiree.DeleteAfter.Time)
	require.False(t, rotator.InitializeFallbackDeadlines(cr, handover.Add(time.Hour)), "a later reconcile must not slide the deadline")

	for _, tc := range []struct {
		name    string
		delta   time.Duration
		expired bool
	}{{"before", -time.Second, false}, {"at", 0, true}, {"after", time.Second, true}} {
		t.Run(tc.name, func(t *testing.T) {
			rotator.Now = func() time.Time { return wantDeadline.Add(tc.delta) }
			cf := &fake.MockServiceCredentialBinding{}
			if tc.expired {
				cf.On("Delete", mock.Anything, rotationTestK0).Return("", nil).Once()
			}
			rotator.SCBClient = cf
			require.Equal(t, tc.expired, rotator.HasExpiredKeys(cr))
			remaining, err := rotator.DeleteExpiredKeys(context.Background(), cr)
			require.NoError(t, err)
			if tc.expired {
				require.Empty(t, remaining)
			} else {
				require.Equal(t, []*v1alpha1.SCBResource{retiree}, remaining)
			}
			cf.AssertExpectations(t)
		})
	}
}

func TestRotationFallbackRetentionFrequencyOnly(t *testing.T) {
	now := time.Date(2026, time.October, 28, 9, 0, 0, 0, time.UTC)
	cr := rotationTestCR("key")
	cr.Spec.ForProvider.Rotation = &v1alpha1.RotationParameters{Frequency: &metav1.Duration{Duration: time.Hour}}
	cr.Status.AtProvider.RetiredKeys = []*v1alpha1.SCBResource{{
		GUID: rotationTestK0, DeleteAfter: ptr.To(metav1.NewTime(now.Add(DefaultRetiredKeyRetention))),
	}}
	rotator := &SCBKeyRotator{Now: func() time.Time { return now }}
	require.False(t, rotator.HasExpiredKeys(cr))
	rotator.Now = func() time.Time { return now.Add(DefaultRetiredKeyRetention) }
	require.True(t, rotator.HasExpiredKeys(cr), "frequency-only configuration uses the fallback deadline")
}

func TestRotationFallbackRetentionLegacyAndActiveProtection(t *testing.T) {
	now := time.Date(2026, time.October, 28, 9, 0, 0, 0, time.UTC)
	legacy := &v1alpha1.SCBResource{GUID: rotationTestK0, CreatedAt: ptr.To(metav1.NewTime(now.Add(-180 * 24 * time.Hour)))}
	active := &v1alpha1.SCBResource{GUID: rotationTestK1}
	cr := rotationTestCR("key")
	cr.Annotations = map[string]string{"crossplane.io/external-name": rotationTestK1}
	cr.Status.AtProvider.GUID = rotationTestK1
	cr.Status.AtProvider.RetiredKeys = []*v1alpha1.SCBResource{legacy, active, nil, {Name: "unknown"}}
	rotator := &SCBKeyRotator{Now: func() time.Time { return now }}

	require.True(t, rotator.InitializeFallbackDeadlines(cr, now))
	require.NotNil(t, legacy.DeleteAfter)
	require.Nil(t, active.DeleteAfter, "the active identity must not receive a fallback deadline")
	require.Nil(t, cr.Status.AtProvider.RetiredKeys[2])
	require.Nil(t, cr.Status.AtProvider.RetiredKeys[3].DeleteAfter, "unknown identities must not receive guessed deadlines")

	// Removing an explicit TTL uses a persisted fallback deadline; while TTL is
	// configured, the historical createdAt+TTL contract remains authoritative.
	cr.Spec.ForProvider.Rotation = &v1alpha1.RotationParameters{TTL: &metav1.Duration{Duration: time.Hour}}
	rotator.Now = func() time.Time { return now }
	require.True(t, rotator.HasExpiredKeys(cr), "explicit TTL uses the old creation age, not deleteAfter")
	legacy.CreatedAt = ptr.To(metav1.NewTime(now.Add(-2 * time.Hour)))
	require.True(t, rotator.HasExpiredKeys(cr), "explicit creation-based TTL remains authoritative")
}

func TestRotationExplicitTTLCompatibility(t *testing.T) {
	now := time.Date(2026, time.October, 28, 9, 0, 0, 0, time.UTC)
	retiree := &v1alpha1.SCBResource{GUID: rotationTestK0, CreatedAt: ptr.To(metav1.NewTime(now.Add(-time.Hour)))}
	cr := rotationTestCR("key")
	cr.Spec.ForProvider.Rotation = &v1alpha1.RotationParameters{TTL: &metav1.Duration{Duration: time.Hour}}
	cr.Annotations = map[string]string{"crossplane.io/external-name": rotationTestK1}
	cr.Status.AtProvider.RetiredKeys = []*v1alpha1.SCBResource{retiree}
	rotator := &SCBKeyRotator{Now: func() time.Time { return now }}
	require.True(t, rotator.HasExpiredKeys(cr), "explicit TTL is eligible at createdAt+TTL")

	cr.Spec.ForProvider.Rotation = nil
	require.False(t, rotator.HasExpiredKeys(cr), "removing TTL does not infer fallback from old createdAt")
	require.True(t, rotator.InitializeFallbackDeadlines(cr, now))
	require.False(t, rotator.HasExpiredKeys(cr))
	require.Equal(t, now.Add(DefaultRetiredKeyRetention), retiree.DeleteAfter.Time)
}

func TestSCBKeyRotator_CanonicalUUIDTargetAndPendingContinuation(t *testing.T) {
	cr := rotationTestCR("key")
	cr.Annotations = map[string]string{ForceRotationKey: "{11111111-1111-4111-8111-111111111111}"}
	require.False(t, (&SCBKeyRotator{}).RetireBinding(cr, rotationTestBinding(rotationTestK0)), "braced UUIDs are outside the documented canonical form")

	cr.Annotations[ForceRotationKey] = "11111111-1111-4111-8111-111111111111"
	require.True(t, (&SCBKeyRotator{}).RetireBinding(cr, rotationTestBinding(stringsToUpper(rotationTestK0))))
	cr.Annotations = nil
	require.True(t, (&SCBKeyRotator{}).RetireBinding(cr, rotationTestBinding(rotationTestK0)), "persisted retired identity resumes without an annotation")
}

func stringsToUpper(value string) string {
	return strings.ToUpper(value)
}
