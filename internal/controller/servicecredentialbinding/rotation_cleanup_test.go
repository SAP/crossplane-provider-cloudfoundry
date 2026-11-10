package servicecredentialbinding

import (
	"context"
	"testing"
	"time"

	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/fake"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/servicecredentialbinding"
)

func TestRotationCleanupNilSafety(t *testing.T) {
	retired := &v1alpha1.SCBResource{
		GUID:      rotationTestK0,
		CreatedAt: &metav1.Time{Time: time.Now().Add(-48 * time.Hour)},
	}
	cases := map[string]struct {
		rotation *v1alpha1.RotationParameters
		retired  []*v1alpha1.SCBResource
	}{
		"rotation absent": {retired: []*v1alpha1.SCBResource{retired}},
		"ttl absent": {
			rotation: &v1alpha1.RotationParameters{Frequency: &metav1.Duration{Duration: time.Hour}},
			retired:  []*v1alpha1.SCBResource{retired},
		},
		"nil entry":         {retired: []*v1alpha1.SCBResource{nil}},
		"missing timestamp": {retired: []*v1alpha1.SCBResource{{GUID: rotationTestK0}}},
		"empty identity":    {retired: []*v1alpha1.SCBResource{{CreatedAt: retired.CreatedAt}}},
		"empty list":        {retired: []*v1alpha1.SCBResource{}},
		"nil list":          {retired: nil},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cr := serviceCredentialBinding("key", withServiceInstanceID(serviceInstanceGUID), withExternalName(rotationTestK1))
			cr.Spec.ForProvider.Rotation = tc.rotation
			cr.Status.AtProvider.RetiredKeys = tc.retired
			cf := &fake.MockServiceCredentialBinding{}
			cf.On("Update", mock.Anything, rotationTestK1, mock.Anything).Return(
				&cfresource.ServiceCredentialBinding{Resource: cfresource.Resource{GUID: rotationTestK1}}, nil,
			)
			cf.On("Get", mock.Anything, rotationTestK1).Return(rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationSucceeded), nil).Maybe()
			rotator := &servicecredentialbinding.SCBKeyRotator{SCBClient: cf}
			ext := &external{
				kube: &test.MockClient{
					MockUpdate:       test.NewMockUpdateFn(nil),
					MockStatusUpdate: func(context.Context, k8s.Object, ...k8s.SubResourceUpdateOption) error { return nil },
				},
				scbClient:  cf,
				keyRotator: rotator,
			}

			assert.NotPanics(t, func() {
				_, err := ext.Update(context.Background(), cr)
				assert.NoError(t, err)
			})
			cf.AssertNotCalled(t, "Delete", mock.Anything, "")
			cf.AssertExpectations(t)
		})
	}
}

func TestRotationCleanupPartialProgress(t *testing.T) {
	removed := &v1alpha1.SCBResource{GUID: rotationTestK0, CreatedAt: &metav1.Time{Time: time.Now().Add(-48 * time.Hour)}}
	failed := &v1alpha1.SCBResource{GUID: "33333333-3333-4333-8333-333333333333", CreatedAt: &metav1.Time{Time: time.Now().Add(-48 * time.Hour)}}
	cr := serviceCredentialBinding("key", withServiceInstanceID(serviceInstanceGUID), withExternalName(rotationTestK1))
	cr.Spec.ForProvider.Rotation = &v1alpha1.RotationParameters{TTL: &metav1.Duration{Duration: time.Hour}}
	cr.Status.AtProvider.RetiredKeys = []*v1alpha1.SCBResource{removed, failed}
	cf := &fake.MockServiceCredentialBinding{}
	cf.On("Update", mock.Anything, rotationTestK1, mock.Anything).Return(
		&cfresource.ServiceCredentialBinding{Resource: cfresource.Resource{GUID: rotationTestK1}}, nil,
	)
	cf.On("Get", mock.Anything, rotationTestK1).Return(rotationControllerBinding(rotationTestK1, v1alpha1.LastOperationSucceeded), nil)
	cf.On("Get", mock.Anything, removed.GUID).Return(rotationControllerBinding(removed.GUID, v1alpha1.LastOperationSucceeded), nil).Once()
	cf.On("Delete", mock.Anything, removed.GUID).Return("", nil).Once()
	cf.On("Get", mock.Anything, removed.GUID).Return((*cfresource.ServiceCredentialBinding)(nil), cfresource.NewResourceNotFoundError()).Once()
	cf.On("Get", mock.Anything, failed.GUID).Return(rotationControllerBinding(failed.GUID, v1alpha1.LastOperationSucceeded), nil).Once()
	cf.On("Delete", mock.Anything, failed.GUID).Return("", assert.AnError).Once()
	store := newRotationStoredClient(cr)
	ext := &external{
		kube:       store,
		scbClient:  cf,
		keyRotator: &servicecredentialbinding.SCBKeyRotator{SCBClient: cf},
	}

	_, err := ext.Update(context.Background(), cr)
	require.ErrorContains(t, err, "cannot delete expired key "+failed.GUID+": cannot delete service credential binding "+failed.GUID+": assert.AnError general error for testing")
	assert.Equal(t, []*v1alpha1.SCBResource{failed}, cr.Status.AtProvider.RetiredKeys,
		"successfully deleted and not-found keys should be removed even when another deletion fails")
	assert.Equal(t, []*v1alpha1.SCBResource{failed}, store.get().Status.AtProvider.RetiredKeys,
		"successful deletion progress must be persisted even when another deletion fails")
	cf.AssertExpectations(t)
}
