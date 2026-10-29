package servicecredentialbinding

import (
	"context"
	"testing"
	"time"

	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/fake"
)

func TestSCBKeyRotator_ActiveKeyProtection(t *testing.T) {
	active := &v1alpha1.SCBResource{
		GUID:      "11111111-1111-4111-8111-111111111111",
		CreatedAt: &metav1.Time{Time: time.Now().Add(-48 * time.Hour)},
	}
	cr := &v1alpha1.ServiceCredentialBinding{
		ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
			"crossplane.io/external-name": active.GUID,
		}},
		Spec: v1alpha1.ServiceCredentialBindingSpec{ForProvider: v1alpha1.ServiceCredentialBindingParameters{
			Rotation: &v1alpha1.RotationParameters{TTL: &metav1.Duration{Duration: time.Hour}},
		}},
		Status: v1alpha1.ServiceCredentialBindingStatus{AtProvider: v1alpha1.ServiceCredentialBindingObservation{
			RetiredKeys: []*v1alpha1.SCBResource{active},
		}},
	}

	rotator := &SCBKeyRotator{}
	assert.False(t, rotator.HasExpiredKeys(cr), "the active GUID must not report routine cleanup work")

	cf := &fake.MockServiceCredentialBinding{}
	rotator.SCBClient = cf
	remaining, err := rotator.DeleteExpiredKeys(context.Background(), cr)
	assert.NoError(t, err)
	assert.Equal(t, []*v1alpha1.SCBResource{active}, remaining)
	cf.AssertNotCalled(t, "Delete", mock.Anything, active.GUID)
}

func TestSCBKeyRotator_CleanupMalformedRetiredEntries(t *testing.T) {
	old := &metav1.Time{Time: time.Now().Add(-48 * time.Hour)}
	cases := map[string][]*v1alpha1.SCBResource{
		"nil rotation":             {{GUID: "retired", CreatedAt: old}},
		"frequency without ttl":    {{GUID: "retired", CreatedAt: old}},
		"nil entry":                {nil},
		"missing creation time":    {{GUID: "retired"}},
		"empty guid":               {{CreatedAt: old}},
		"nil active creation time": {{GUID: "active"}},
	}

	for name, retired := range cases {
		t.Run(name, func(t *testing.T) {
			cr := &v1alpha1.ServiceCredentialBinding{
				Spec: v1alpha1.ServiceCredentialBindingSpec{ForProvider: v1alpha1.ServiceCredentialBindingParameters{}},
				Status: v1alpha1.ServiceCredentialBindingStatus{AtProvider: v1alpha1.ServiceCredentialBindingObservation{
					RetiredKeys: retired,
				}},
			}
			if name == "frequency without ttl" {
				cr.Spec.ForProvider.Rotation = &v1alpha1.RotationParameters{Frequency: &metav1.Duration{Duration: time.Hour}}
			}
			if name == "nil active creation time" {
				cr.Annotations = map[string]string{"crossplane.io/external-name": "active"}
				retired[0].GUID = "active"
			}
			cf := &fake.MockServiceCredentialBinding{}
			rotator := &SCBKeyRotator{SCBClient: cf}

			assert.NotPanics(t, func() {
				remaining, err := rotator.DeleteExpiredKeys(context.Background(), cr)
				assert.NoError(t, err)
				assert.Equal(t, retired, remaining, "unknown expiry must conservatively retain its identity")
			})
			cf.AssertNotCalled(t, "Delete", mock.Anything, "")
		})
	}
}

func TestSCBKeyRotator_DeleteExpiredKeysPartialProgress(t *testing.T) {
	expired := func(guid string) *v1alpha1.SCBResource {
		return &v1alpha1.SCBResource{GUID: guid, CreatedAt: &metav1.Time{Time: time.Now().Add(-48 * time.Hour)}}
	}
	removed := expired("removed")
	missingResource := expired("missing-resource")
	missingBinding := expired("missing-binding")
	failed := expired("failed")
	cr := &v1alpha1.ServiceCredentialBinding{
		Spec: v1alpha1.ServiceCredentialBindingSpec{ForProvider: v1alpha1.ServiceCredentialBindingParameters{
			Rotation: &v1alpha1.RotationParameters{TTL: &metav1.Duration{Duration: time.Hour}},
		}},
		Status: v1alpha1.ServiceCredentialBindingStatus{AtProvider: v1alpha1.ServiceCredentialBindingObservation{
			RetiredKeys: []*v1alpha1.SCBResource{removed, missingResource, missingBinding, failed},
		}},
	}
	cf := &fake.MockServiceCredentialBinding{}
	cf.On("Delete", mock.Anything, removed.GUID).Return("", nil)
	cf.On("Delete", mock.Anything, missingResource.GUID).Return("", cfresource.NewResourceNotFoundError())
	cf.On("Delete", mock.Anything, missingBinding.GUID).Return("", cfresource.NewServiceBindingNotFoundError())
	cf.On("Delete", mock.Anything, failed.GUID).Return("", assert.AnError)

	remaining, err := (&SCBKeyRotator{SCBClient: cf}).DeleteExpiredKeys(context.Background(), cr)
	assert.EqualError(t, err, "cannot delete expired key failed: assert.AnError general error for testing")
	assert.Equal(t, []*v1alpha1.SCBResource{failed}, remaining)
	cf.AssertExpectations(t)
}

func TestSCBKeyRotator_DeleteExpiredKeysKeepsCurrentWithMixedKeys(t *testing.T) {
	active := &v1alpha1.SCBResource{GUID: "current", CreatedAt: &metav1.Time{Time: time.Now().Add(-48 * time.Hour)}}
	old := &v1alpha1.SCBResource{GUID: "old", CreatedAt: &metav1.Time{Time: time.Now().Add(-48 * time.Hour)}}
	cr := &v1alpha1.ServiceCredentialBinding{
		ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"crossplane.io/external-name": active.GUID}},
		Spec: v1alpha1.ServiceCredentialBindingSpec{ForProvider: v1alpha1.ServiceCredentialBindingParameters{
			Rotation: &v1alpha1.RotationParameters{TTL: &metav1.Duration{Duration: time.Hour}},
		}},
		Status: v1alpha1.ServiceCredentialBindingStatus{AtProvider: v1alpha1.ServiceCredentialBindingObservation{
			RetiredKeys: []*v1alpha1.SCBResource{active, old},
		}},
	}
	cf := &fake.MockServiceCredentialBinding{}
	cf.On("Delete", mock.Anything, old.GUID).Return("", nil)
	rotator := &SCBKeyRotator{SCBClient: cf}

	assert.True(t, rotator.HasExpiredKeys(cr), "only the non-active expired key should be eligible")
	remaining, err := rotator.DeleteExpiredKeys(context.Background(), cr)
	assert.NoError(t, err)
	assert.Equal(t, []*v1alpha1.SCBResource{active}, remaining)
	cf.AssertNotCalled(t, "Delete", mock.Anything, active.GUID)
	cf.AssertExpectations(t)
}

func TestSCBKeyRotator_DeleteRetiredKeysNotFoundVariants(t *testing.T) {
	cf := &fake.MockServiceCredentialBinding{}
	cf.On("Delete", mock.Anything, "resource-missing").Return("", cfresource.NewResourceNotFoundError())
	cf.On("Delete", mock.Anything, "binding-missing").Return("", cfresource.NewServiceBindingNotFoundError())
	cr := &v1alpha1.ServiceCredentialBinding{Status: v1alpha1.ServiceCredentialBindingStatus{
		AtProvider: v1alpha1.ServiceCredentialBindingObservation{RetiredKeys: []*v1alpha1.SCBResource{
			{GUID: "resource-missing"}, {GUID: "binding-missing"},
		}},
	}}

	assert.NoError(t, (&SCBKeyRotator{SCBClient: cf}).DeleteRetiredKeys(context.Background(), cr))
	cf.AssertExpectations(t)
}

func TestSCBKeyRotator_DeleteRetiredKeysRejectsUnknownIdentity(t *testing.T) {
	cf := &fake.MockServiceCredentialBinding{}
	cf.On("Delete", mock.Anything, "").Return("", nil)
	cr := &v1alpha1.ServiceCredentialBinding{Status: v1alpha1.ServiceCredentialBindingStatus{
		AtProvider: v1alpha1.ServiceCredentialBindingObservation{RetiredKeys: []*v1alpha1.SCBResource{{Name: "no-guid"}}},
	}}
	assert.NotPanics(t, func() {
		assert.EqualError(t, (&SCBKeyRotator{SCBClient: cf}).DeleteRetiredKeys(context.Background(), cr), "cannot delete retired key with missing GUID")
	})
	cf.AssertNotCalled(t, "Delete", mock.Anything, "")
}
