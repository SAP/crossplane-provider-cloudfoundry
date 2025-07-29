package servicecredentialbinding

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/fake"
)

func TestSCBKeyRotator_RetireBinding(t *testing.T) {
	type args struct {
		cr             *v1alpha1.ServiceCredentialBinding
		serviceBinding *cfresource.ServiceCredentialBinding
	}

	type want struct {
		shouldRetire bool
	}

	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)
	twoHoursAgo := now.Add(-2 * time.Hour)

	// Create test CRs with different rotation configurations
	crWithRotation := &v1alpha1.ServiceCredentialBinding{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{},
		},
		Spec: v1alpha1.ServiceCredentialBindingSpec{
			ForProvider: v1alpha1.ServiceCredentialBindingParameters{
				Rotation: &v1alpha1.RotationParameters{
					Frequency: &metav1.Duration{Duration: 30 * time.Minute},
					TTL:       &metav1.Duration{Duration: 2 * time.Hour},
				},
			},
		},
		Status: v1alpha1.ServiceCredentialBindingStatus{
			AtProvider: v1alpha1.ServiceCredentialBindingObservation{
				SCBResource: v1alpha1.SCBResource{
					CreatedAt: &metav1.Time{Time: twoHoursAgo}, // Created 2 hours ago, should be rotated
				},
			},
		},
	}

	crWithForceRotation := &v1alpha1.ServiceCredentialBinding{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				ForceRotationKey: "true",
			},
		},
		Spec: v1alpha1.ServiceCredentialBindingSpec{
			ForProvider: v1alpha1.ServiceCredentialBindingParameters{},
		},
		Status: v1alpha1.ServiceCredentialBindingStatus{
			AtProvider: v1alpha1.ServiceCredentialBindingObservation{},
		},
	}

	crWithoutRotation := &v1alpha1.ServiceCredentialBinding{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{},
		},
		Spec: v1alpha1.ServiceCredentialBindingSpec{
			ForProvider: v1alpha1.ServiceCredentialBindingParameters{},
		},
		Status: v1alpha1.ServiceCredentialBindingStatus{
			AtProvider: v1alpha1.ServiceCredentialBindingObservation{
				SCBResource: v1alpha1.SCBResource{
					CreatedAt: &metav1.Time{Time: oneHourAgo},
				},
			},
		},
	}

	serviceBindingResource := &cfresource.ServiceCredentialBinding{
		Resource: cfresource.Resource{GUID: "test-guid"},
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"ShouldRetireDueToFrequency": {
			args: args{
				cr:             crWithRotation,
				serviceBinding: serviceBindingResource,
			},
			want: want{
				shouldRetire: true,
			},
		},
		"ShouldRetireDueToForceAnnotation": {
			args: args{
				cr:             crWithForceRotation,
				serviceBinding: serviceBindingResource,
			},
			want: want{
				shouldRetire: true,
			},
		},
		"NoRotationNeeded": {
			args: args{
				cr:             crWithoutRotation,
				serviceBinding: serviceBindingResource,
			},
			want: want{
				shouldRetire: false,
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			rotator := &SCBKeyRotator{}
			shouldRetire := rotator.RetireBinding(tc.args.cr, tc.args.serviceBinding)

			if diff := cmp.Diff(tc.want.shouldRetire, shouldRetire); diff != "" {
				t.Errorf("RetireBinding(...): -want, +got:\n%s", diff)
			}

			// For cases where we expect retirement, verify the key was added to retired list
			if tc.want.shouldRetire && n != "AlreadyRetired" {
				found := false
				for _, retiredKey := range tc.args.cr.Status.AtProvider.RetiredKeys {
					if retiredKey.GUID == serviceBindingResource.GUID {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("RetireBinding(...): expected key to be added to retired list")
				}
			}
		})
	}
}

func TestSCBKeyRotator_HasExpiredKeys(t *testing.T) {
	type args struct {
		cr *v1alpha1.ServiceCredentialBinding
	}

	type want struct {
		hasExpired bool
	}

	now := time.Now()
	twoHoursAgo := now.Add(-2 * time.Hour)
	oneHourAgo := now.Add(-1 * time.Hour)

	crWithExpiredKeys := &v1alpha1.ServiceCredentialBinding{
		Spec: v1alpha1.ServiceCredentialBindingSpec{
			ForProvider: v1alpha1.ServiceCredentialBindingParameters{
				Rotation: &v1alpha1.RotationParameters{
					Frequency: &metav1.Duration{Duration: 30 * time.Minute},
					TTL:       &metav1.Duration{Duration: 1 * time.Hour}, // 1 hour TTL
				},
			},
		},
		Status: v1alpha1.ServiceCredentialBindingStatus{
			AtProvider: v1alpha1.ServiceCredentialBindingObservation{
				RetiredKeys: []*v1alpha1.SCBResource{
					{
						GUID:      "expired-key",
						CreatedAt: &metav1.Time{Time: twoHoursAgo}, // Expired (older than TTL)
					},
				},
			},
		},
	}

	crWithNonExpiredKeys := &v1alpha1.ServiceCredentialBinding{
		Spec: v1alpha1.ServiceCredentialBindingSpec{
			ForProvider: v1alpha1.ServiceCredentialBindingParameters{
				Rotation: &v1alpha1.RotationParameters{
					Frequency: &metav1.Duration{Duration: 30 * time.Minute},
					TTL:       &metav1.Duration{Duration: 2 * time.Hour}, // 2 hour TTL
				},
			},
		},
		Status: v1alpha1.ServiceCredentialBindingStatus{
			AtProvider: v1alpha1.ServiceCredentialBindingObservation{
				RetiredKeys: []*v1alpha1.SCBResource{
					{
						GUID:      "non-expired-key",
						CreatedAt: &metav1.Time{Time: oneHourAgo}, // Not expired yet
					},
				},
			},
		},
	}

	crWithoutRotation := &v1alpha1.ServiceCredentialBinding{
		Spec: v1alpha1.ServiceCredentialBindingSpec{
			ForProvider: v1alpha1.ServiceCredentialBindingParameters{},
		},
		Status: v1alpha1.ServiceCredentialBindingStatus{
			AtProvider: v1alpha1.ServiceCredentialBindingObservation{
				RetiredKeys: []*v1alpha1.SCBResource{
					{
						GUID:      "some-key",
						CreatedAt: &metav1.Time{Time: twoHoursAgo},
					},
				},
			},
		},
	}

	crWithoutRetiredKeys := &v1alpha1.ServiceCredentialBinding{
		Spec: v1alpha1.ServiceCredentialBindingSpec{
			ForProvider: v1alpha1.ServiceCredentialBindingParameters{
				Rotation: &v1alpha1.RotationParameters{
					Frequency: &metav1.Duration{Duration: 30 * time.Minute},
					TTL:       &metav1.Duration{Duration: 1 * time.Hour},
				},
			},
		},
		Status: v1alpha1.ServiceCredentialBindingStatus{
			AtProvider: v1alpha1.ServiceCredentialBindingObservation{},
		},
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"HasExpiredKeys": {
			args: args{
				cr: crWithExpiredKeys,
			},
			want: want{
				hasExpired: true,
			},
		},
		"HasNonExpiredKeys": {
			args: args{
				cr: crWithNonExpiredKeys,
			},
			want: want{
				hasExpired: false,
			},
		},
		"NoRotationConfig": {
			args: args{
				cr: crWithoutRotation,
			},
			want: want{
				hasExpired: false,
			},
		},
		"NoRetiredKeys": {
			args: args{
				cr: crWithoutRetiredKeys,
			},
			want: want{
				hasExpired: false,
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			rotator := &SCBKeyRotator{}
			hasExpired := rotator.HasExpiredKeys(tc.args.cr)

			if diff := cmp.Diff(tc.want.hasExpired, hasExpired); diff != "" {
				t.Errorf("HasExpiredKeys(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestSCBKeyRotator_DeleteExpiredKeys(t *testing.T) {
	type args struct {
		ctx context.Context
		cr  *v1alpha1.ServiceCredentialBinding
	}

	type want struct {
		newRetiredKeys []*v1alpha1.SCBResource
		err            error
	}

	now := time.Now()
	twoHoursAgo := now.Add(-2 * time.Hour)
	oneHourAgo := now.Add(-1 * time.Hour)

	expiredKey := &v1alpha1.SCBResource{
		GUID:      "expired-key",
		CreatedAt: &metav1.Time{Time: twoHoursAgo},
	}

	nonExpiredKey := &v1alpha1.SCBResource{
		GUID:      "non-expired-key",
		CreatedAt: &metav1.Time{Time: oneHourAgo},
	}

	currentKey := &v1alpha1.SCBResource{
		GUID:      "current-key", // This should match external name
		CreatedAt: &metav1.Time{Time: oneHourAgo},
	}

	crWithMixedKeys := &v1alpha1.ServiceCredentialBinding{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"crossplane.io/external-name": "current-key",
			},
		},
		Spec: v1alpha1.ServiceCredentialBindingSpec{
			ForProvider: v1alpha1.ServiceCredentialBindingParameters{
				Rotation: &v1alpha1.RotationParameters{
					Frequency: &metav1.Duration{Duration: 30 * time.Minute},
					TTL:       &metav1.Duration{Duration: 90 * time.Minute}, // 1.5 hour TTL
				},
			},
		},
		Status: v1alpha1.ServiceCredentialBindingStatus{
			AtProvider: v1alpha1.ServiceCredentialBindingObservation{
				RetiredKeys: []*v1alpha1.SCBResource{expiredKey, nonExpiredKey, currentKey},
			},
		},
	}

	cases := map[string]struct {
		args       args
		want       want
		mockClient func() *fake.MockServiceCredentialBinding
	}{
		"DeleteExpiredKeysSuccessfully": {
			args: args{
				ctx: context.Background(),
				cr:  crWithMixedKeys,
			},
			want: want{
				newRetiredKeys: []*v1alpha1.SCBResource{nonExpiredKey, currentKey}, // Only non-expired and current key remain
				err:            nil,
			},
			mockClient: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				m.On("Delete", mock.Anything, "expired-key").Return("", nil)
				return m
			},
		},
		"DeleteFailsForSomeKeys": {
			args: args{
				ctx: context.Background(),
				cr:  crWithMixedKeys,
			},
			want: want{
				newRetiredKeys: []*v1alpha1.SCBResource{expiredKey, nonExpiredKey, currentKey}, // All keys remain due to error
				err:            errors.New("cannot delete expired key expired-key: boom"),
			},
			mockClient: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				m.On("Delete", mock.Anything, "expired-key").Return("", errBoom)
				return m
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			mockClient := tc.mockClient()
			rotator := &SCBKeyRotator{
				SCBClient: mockClient,
			}

			newRetiredKeys, err := rotator.DeleteExpiredKeys(tc.args.ctx, tc.args.cr)

			if tc.want.err != nil && err != nil {
				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("DeleteExpiredKeys(...): want error string != got error string:\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(tc.want.err, err); diff != "" {
					t.Errorf("DeleteExpiredKeys(...): want error != got error:\n%s", diff)
				}
			}

			if diff := cmp.Diff(tc.want.newRetiredKeys, newRetiredKeys); diff != "" {
				t.Errorf("DeleteExpiredKeys(...): -want, +got:\n%s", diff)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

func TestSCBKeyRotator_DeleteRetiredKeys(t *testing.T) {
	type args struct {
		ctx context.Context
		cr  *v1alpha1.ServiceCredentialBinding
	}

	type want struct {
		err error
	}

	retiredKey1 := &v1alpha1.SCBResource{
		GUID:      "retired-key-1",
		CreatedAt: &metav1.Time{Time: time.Now().Add(-1 * time.Hour)},
	}

	retiredKey2 := &v1alpha1.SCBResource{
		GUID:      "retired-key-2",
		CreatedAt: &metav1.Time{Time: time.Now().Add(-2 * time.Hour)},
	}

	crWithRetiredKeys := &v1alpha1.ServiceCredentialBinding{
		Status: v1alpha1.ServiceCredentialBindingStatus{
			AtProvider: v1alpha1.ServiceCredentialBindingObservation{
				RetiredKeys: []*v1alpha1.SCBResource{retiredKey1, retiredKey2},
			},
		},
	}

	cases := map[string]struct {
		args       args
		want       want
		mockClient func() *fake.MockServiceCredentialBinding
	}{
		"DeleteAllRetiredKeysSuccessfully": {
			args: args{
				ctx: context.Background(),
				cr:  crWithRetiredKeys,
			},
			want: want{
				err: nil,
			},
			mockClient: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				m.On("Delete", mock.Anything, "retired-key-1").Return("", nil)
				m.On("Delete", mock.Anything, "retired-key-2").Return("", nil)
				return m
			},
		},
		"DeleteFailsForOneKey": {
			args: args{
				ctx: context.Background(),
				cr:  crWithRetiredKeys,
			},
			want: want{
				err: errors.New("cannot delete retired key retired-key-1: boom"),
			},
			mockClient: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				m.On("Delete", mock.Anything, "retired-key-1").Return("", errBoom)
				return m
			},
		},
		"IgnoreNotFoundErrors": {
			args: args{
				ctx: context.Background(),
				cr:  crWithRetiredKeys,
			},
			want: want{
				err: nil,
			},
			mockClient: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				// Simulate resource not found errors which should be ignored
				notFoundErr := cfresource.NewResourceNotFoundError()
				m.On("Delete", mock.Anything, "retired-key-1").Return("", notFoundErr)
				m.On("Delete", mock.Anything, "retired-key-2").Return("", nil)
				return m
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			mockClient := tc.mockClient()
			rotator := &SCBKeyRotator{
				SCBClient: mockClient,
			}

			err := rotator.DeleteRetiredKeys(tc.args.ctx, tc.args.cr)

			if tc.want.err != nil && err != nil {
				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("DeleteRetiredKeys(...): want error string != got error string:\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(tc.want.err, err); diff != "" {
					t.Errorf("DeleteRetiredKeys(...): want error != got error:\n%s", diff)
				}
			}

			mockClient.AssertExpectations(t)
		})
	}
}
