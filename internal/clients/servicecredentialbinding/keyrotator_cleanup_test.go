package servicecredentialbinding

import (
	"context"
	"testing"
	"time"

	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err)
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
	expectDeleteConfirmed(cf, removed.GUID)
	cf.On("Get", mock.Anything, missingResource.GUID).Return(deleteTestBinding(missingResource.GUID, "create", "succeeded"), nil).Once()
	cf.On("Delete", mock.Anything, missingResource.GUID).Return("", cfresource.NewResourceNotFoundError()).Once()
	cf.On("Get", mock.Anything, missingBinding.GUID).Return(deleteTestBinding(missingBinding.GUID, "create", "succeeded"), nil).Once()
	cf.On("Delete", mock.Anything, missingBinding.GUID).Return("", cfresource.NewServiceBindingNotFoundError()).Once()
	cf.On("Get", mock.Anything, failed.GUID).Return(deleteTestBinding(failed.GUID, "create", "succeeded"), nil).Once()
	cf.On("Delete", mock.Anything, failed.GUID).Return("", assert.AnError).Once()

	remaining, err := (&SCBKeyRotator{SCBClient: cf}).DeleteExpiredKeys(context.Background(), cr)
	require.EqualError(t, err, "cannot delete expired key failed: cannot delete service credential binding failed: assert.AnError general error for testing")
	assert.Equal(t, []*v1alpha1.SCBResource{failed}, remaining)
	cf.AssertExpectations(t)
}

func TestSCBKeyRotator_AsyncDeletionRetainsIdentityUntilNotFound(t *testing.T) {
	retiree := &v1alpha1.SCBResource{GUID: rotationTestK0, CreatedAt: &metav1.Time{Time: time.Now().Add(-48 * time.Hour)}}
	cr := rotationTestCR("key")
	cr.Spec.ForProvider.Rotation = &v1alpha1.RotationParameters{TTL: &metav1.Duration{Duration: time.Hour}}
	cr.Status.AtProvider.RetiredKeys = []*v1alpha1.SCBResource{retiree}
	cf := &fake.MockServiceCredentialBinding{}
	cf.On("Get", mock.Anything, retiree.GUID).Return(deleteTestBinding(retiree.GUID, "create", "succeeded"), nil).Once()
	cf.On("Delete", mock.Anything, retiree.GUID).Return("job-guid", nil).Once()
	cf.On("Get", mock.Anything, retiree.GUID).Return(deleteTestBinding(retiree.GUID, "delete", "in progress"), nil).Once()
	cf.On("Get", mock.Anything, retiree.GUID).Return((*cfresource.ServiceCredentialBinding)(nil), cfresource.NewResourceNotFoundError()).Once()
	rotator := &SCBKeyRotator{SCBClient: cf}

	remaining, err := rotator.DeleteExpiredKeys(context.Background(), cr)
	require.ErrorIs(t, err, ErrDeletionPending)
	require.Equal(t, []*v1alpha1.SCBResource{retiree}, remaining, "accepted async deletion must not discard tracking")
	require.Same(t, retiree, remaining[0], "retired identity and its deadline remain unchanged")

	remaining, err = rotator.DeleteExpiredKeys(context.Background(), cr)
	require.NoError(t, err)
	require.Empty(t, remaining, "a later binding GET not-found confirms completion")
	cf.AssertNumberOfCalls(t, "Delete", 1)
	cf.AssertExpectations(t)
}

func TestSCBKeyRotator_FullDeletionWaitsForAsyncRetiredKey(t *testing.T) {
	retiree := &v1alpha1.SCBResource{GUID: rotationTestK0}
	cr := &v1alpha1.ServiceCredentialBinding{Status: v1alpha1.ServiceCredentialBindingStatus{
		AtProvider: v1alpha1.ServiceCredentialBindingObservation{RetiredKeys: []*v1alpha1.SCBResource{retiree}},
	}}
	cf := &fake.MockServiceCredentialBinding{}
	cf.On("Get", mock.Anything, retiree.GUID).Return(deleteTestBinding(retiree.GUID, "create", "succeeded"), nil).Once()
	cf.On("Delete", mock.Anything, retiree.GUID).Return("job-guid", nil).Once()
	cf.On("Get", mock.Anything, retiree.GUID).Return(deleteTestBinding(retiree.GUID, "delete", "in progress"), nil).Once()

	err := (&SCBKeyRotator{SCBClient: cf}).DeleteRetiredKeys(context.Background(), cr)
	require.ErrorIs(t, err, ErrDeletionPending)
	require.Equal(t, []*v1alpha1.SCBResource{retiree}, cr.Status.AtProvider.RetiredKeys, "full deletion must remain incomplete while the retiree exists")
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
	expectDeleteConfirmed(cf, old.GUID)
	rotator := &SCBKeyRotator{SCBClient: cf}

	assert.True(t, rotator.HasExpiredKeys(cr), "only the non-active expired key should be eligible")
	remaining, err := rotator.DeleteExpiredKeys(context.Background(), cr)
	require.NoError(t, err)
	assert.Equal(t, []*v1alpha1.SCBResource{active}, remaining)
	cf.AssertNotCalled(t, "Delete", mock.Anything, active.GUID)
	cf.AssertExpectations(t)
}

func TestSCBKeyRotator_DeleteRetiredKeysNotFoundVariants(t *testing.T) {
	cf := &fake.MockServiceCredentialBinding{}
	cf.On("Get", mock.Anything, "resource-missing").Return(deleteTestBinding("resource-missing", "create", "succeeded"), nil).Once()
	cf.On("Delete", mock.Anything, "resource-missing").Return("", cfresource.NewResourceNotFoundError()).Once()
	cf.On("Get", mock.Anything, "binding-missing").Return(deleteTestBinding("binding-missing", "create", "succeeded"), nil).Once()
	cf.On("Delete", mock.Anything, "binding-missing").Return("", cfresource.NewServiceBindingNotFoundError()).Once()
	cr := &v1alpha1.ServiceCredentialBinding{Status: v1alpha1.ServiceCredentialBindingStatus{
		AtProvider: v1alpha1.ServiceCredentialBindingObservation{RetiredKeys: []*v1alpha1.SCBResource{
			{GUID: "resource-missing"}, {GUID: "binding-missing"},
		}},
	}}

	require.NoError(t, (&SCBKeyRotator{SCBClient: cf}).DeleteRetiredKeys(context.Background(), cr))
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
