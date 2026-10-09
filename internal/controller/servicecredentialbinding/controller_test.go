package servicecredentialbinding

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strconv"
	"strings"
	"testing"
	"time"

	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/mock"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/fake"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/servicecredentialbinding"
)

var (
	errCFClientError          = errors.New("boom")
	errServiceInstanceMissing = errors.New(servicecredentialbinding.ErrServiceInstanceMissing)
	errAppMissing             = errors.New(servicecredentialbinding.ErrAppMissing)
	name                      = "my-service-credential-binding"
	guid                      = "2d8b0d04-d537-4e4e-8c6f-f09ca0e7f56f"
	serviceInstanceGUID       = "3d8b0d04-d537-4e4e-8c6f-f09ca0e7f56f"
)

// MockObservationStateHandler is a mock implementation of ObservationStateHandler
type MockObservationStateHandler struct {
	mock.Mock
}

func (m *MockObservationStateHandler) HandleObservationState(serviceBinding *cfresource.ServiceCredentialBinding, ctx context.Context, cr *v1alpha1.ServiceCredentialBinding) (managed.ExternalObservation, error) {
	args := m.Called(serviceBinding, ctx, cr)
	return args.Get(0).(managed.ExternalObservation), args.Error(1)
}

type modifier func(*v1alpha1.ServiceCredentialBinding)

func withExternalName(name string) modifier {
	return func(r *v1alpha1.ServiceCredentialBinding) {
		r.Annotations[meta.AnnotationKeyExternalName] = name
	}
}

func withServiceInstanceID(guid string) modifier {
	return func(r *v1alpha1.ServiceCredentialBinding) {
		r.Spec.ForProvider.ServiceInstance = &guid
	}
}

func withConditions(c ...xpv1.Condition) modifier {
	return func(i *v1alpha1.ServiceCredentialBinding) { i.Status.SetConditions(c...) }
}

func withStatus(guid string) modifier {
	o := v1alpha1.ServiceCredentialBindingObservation{}
	o.GUID = guid

	return func(r *v1alpha1.ServiceCredentialBinding) {
		r.Status.AtProvider = o
	}
}

func withObservedAnnotations(annotations map[string]*string) modifier {
	return func(r *v1alpha1.ServiceCredentialBinding) {
		r.Status.AtProvider.Annotations = annotations
	}
}

func withObservation(guid string, lastOp *v1alpha1.LastOperation) modifier {
	return func(r *v1alpha1.ServiceCredentialBinding) {
		r.Status.AtProvider.GUID = guid
		r.Status.AtProvider.Name = name
		r.Status.AtProvider.CreatedAt = &metav1.Time{}
		r.Status.AtProvider.LastOperation = lastOp
	}
}

func serviceCredentialBinding(typ string, m ...modifier) *v1alpha1.ServiceCredentialBinding {
	r := &v1alpha1.ServiceCredentialBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Finalizers:  []string{},
			Annotations: map[string]string{},
		},
		Spec: v1alpha1.ServiceCredentialBindingSpec{
			ForProvider: v1alpha1.ServiceCredentialBindingParameters{Type: typ, Name: &name, ServiceInstanceRef: &xpv1.Reference{}},
		},
		Status: v1alpha1.ServiceCredentialBindingStatus{
			AtProvider: v1alpha1.ServiceCredentialBindingObservation{},
		},
	}

	for _, rm := range m {
		rm(r)
	}
	return r
}
func withDefaultMetadata() modifier {
	return func(r *v1alpha1.ServiceCredentialBinding) {
		r.SetGroupVersionKind(v1alpha1.ServiceCredentialBindingGroupVersionKind)
	}
}

func TestObserve(t *testing.T) {
	type service func() *fake.MockServiceCredentialBinding
	type keyRotator func() *fake.MockKeyRotator
	type observationStateHandler func() *MockObservationStateHandler
	type args struct {
		mg resource.Managed
	}

	type want struct {
		mg  resource.Managed
		obs managed.ExternalObservation
		err error
		// tripped: the kube Update received the marker and no counter.
		tripped bool
	}

	scb := serviceCredentialBinding("key", withExternalName(guid), withServiceInstanceID(serviceInstanceGUID), withDefaultMetadata())

	cfSucceeded := func() *cfresource.ServiceCredentialBinding {
		return &fake.NewServiceCredentialBinding("key").SetName(name).SetGUID(guid).SetServiceInstanceRef(serviceInstanceGUID).SetLastOperation(v1alpha1.LastOperationCreate, v1alpha1.LastOperationSucceeded).SetAnnotations(map[string]*string{"crossplane-kind": ptr.To("servicecredentialbinding.cloudfoundry.crossplane.io"), "crossplane-name": ptr.To("my-service-credential-binding")}).ServiceCredentialBinding
	}
	cfNotFound := func() *fake.MockServiceCredentialBinding {
		m := &fake.MockServiceCredentialBinding{}
		m.On("Get", mock.Anything, guid).Return(
			fake.ServiceCredentialBindingNil,
			fake.ErrNoResultReturned,
		)
		return m
	}
	noKeyRotator := func() *fake.MockKeyRotator { return &fake.MockKeyRotator{} }
	deletingOverLimit := serviceCredentialBinding("key",
		withExternalName(guid),
		withServiceInstanceID(serviceInstanceGUID),
		withDeletionTimestamp(),
		withCreateAttempts(DefaultMaxCreateAttempts),
	)

	cases := map[string]struct {
		args                    args
		want                    want
		service                 service
		kube                    k8s.Client
		keyRotator              keyRotator
		observationStateHandler observationStateHandler
		limit                   int // 0 means DefaultMaxCreateAttempts
		wantDeleteRetiredKeys   int
		wantNoRetire            bool
		updateErr               error
		// wantKubeCalls is checked only on the default kube client.
		wantKubeCalls []string
		wantEvents    []string
	}{
		"Nil": {
			args: args{
				mg: nil,
			},
			want: want{
				obs: managed.ExternalObservation{ResourceExists: false},
				err: errors.New(errWrongCRType),
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				return m
			},
			keyRotator: func() *fake.MockKeyRotator {
				m := &fake.MockKeyRotator{}
				return m
			},
		},
		"ExternalNameNotSet": {
			args: args{
				mg: scb.DeepCopy(),
			},
			want: want{
				mg: scb.DeepCopy(),
				obs: managed.ExternalObservation{
					ResourceExists: false,
				},
				err: nil,
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				m.On("Single").Return(
					fake.ServiceCredentialBindingNil,
					fake.ErrNoResultReturned,
				)
				m.On("Get", mock.Anything, guid).Return(
					fake.ServiceCredentialBindingNil,
					fake.ErrNoResultReturned,
				)
				return m
			},
			keyRotator: func() *fake.MockKeyRotator {
				m := &fake.MockKeyRotator{}
				return m
			},
		},
		"CFClientError": {
			args: args{
				mg: scb.DeepCopy(),
			},
			want: want{
				obs: managed.ExternalObservation{},
				err: fmt.Errorf(errGet, errCFClientError),
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				m.On("Get", mock.Anything, guid).Return(
					fake.ServiceCredentialBindingNil,
					errCFClientError,
				)
				m.On("Single").Return(
					fake.ServiceCredentialBindingNil,
					errCFClientError,
				)
				return m
			},
			keyRotator: func() *fake.MockKeyRotator {
				m := &fake.MockKeyRotator{}
				return m
			},
		},
		"NotFound": {
			args: args{
				mg: scb.DeepCopy(),
			},
			want: want{
				obs: managed.ExternalObservation{ResourceExists: false},
				err: nil,
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				m.On("Get", mock.Anything, guid).Return(
					fake.ServiceCredentialBindingNil,
					fake.ErrNoResultReturned,
				)
				m.On("Single").Return(
					fake.ServiceCredentialBindingNil,
					fake.ErrNoResultReturned,
				)
				return m
			},
			kube: &test.MockClient{},
			keyRotator: func() *fake.MockKeyRotator {
				m := &fake.MockKeyRotator{}
				return m
			},
		},
		"Successful": {
			args: args{
				mg: scb.DeepCopy(),
			},
			want: want{
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true, ConnectionDetails: managed.ConnectionDetails{}},
				err: nil,
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				m.On("Get", mock.Anything, guid).Return(
					cfSucceeded(),
					nil,
				)
				m.On("Single").Return(
					cfSucceeded(),
					nil,
				)
				m.On("GetDetails", guid).Return(
					fake.NewServiceCredentialBindingDetails(guid),
					nil,
				)
				return m
			},
			keyRotator: func() *fake.MockKeyRotator {
				m := &fake.MockKeyRotator{}
				m.On("HasExpiredKeys", scb.DeepCopy()).Return(false)
				m.On("RetireBinding", mock.Anything, mock.Anything).Return(false)
				return m
			},
			observationStateHandler: func() *MockObservationStateHandler {
				m := &MockObservationStateHandler{}
				m.On("HandleObservationState", cfSucceeded(), mock.Anything, mock.Anything).Return(
					managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true, ConnectionDetails: managed.ConnectionDetails{}},
					nil,
				)
				return m
			},
		},
		"ObservationStateHandlerCalled": {
			args: args{
				mg: scb.DeepCopy(),
			},
			want: want{
				mg: serviceCredentialBinding("key",
					withExternalName(guid), withServiceInstanceID(serviceInstanceGUID),
					withDefaultMetadata(),
					withObservation(guid, &v1alpha1.LastOperation{
						Type:        "create",
						State:       "succeeded",
						Description: "create succeeded",
						CreatedAt:   "0001-01-01 00:00:00 +0000 UTC",
						UpdatedAt:   "",
					}),
					withObservedAnnotations(map[string]*string{
						"crossplane-kind": ptr.To("servicecredentialbinding.cloudfoundry.crossplane.io"),
						"crossplane-name": ptr.To("my-service-credential-binding"),
					}),
				),
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
				err: nil,
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				m.On("Get", mock.Anything, guid).Return(
					cfSucceeded(),
					nil,
				)
				m.On("Single").Return(
					cfSucceeded(),
					nil,
				)
				return m
			},
			keyRotator: func() *fake.MockKeyRotator {
				m := &fake.MockKeyRotator{}
				m.On("RetireBinding", mock.Anything, mock.Anything).Return(false)
				return m
			},
			observationStateHandler: func() *MockObservationStateHandler {
				m := &MockObservationStateHandler{}
				m.On("HandleObservationState", cfSucceeded(), mock.Anything, mock.Anything).Return(
					managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
					nil,
				)
				return m
			},
		},
		"Trip_NotFound_LegacyCounter": {
			// Also the upgrade path from the old counter-only breaker.
			args: args{
				mg: serviceCredentialBinding("key",
					withExternalName(guid),
					withServiceInstanceID(serviceInstanceGUID),
					withCreateAttempts(DefaultMaxCreateAttempts),
				),
			},
			want: want{
				obs:     managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
				tripped: true,
			},
			service:       cfNotFound,
			keyRotator:    noKeyRotator,
			wantKubeCalls: []string{"Update"},
			wantEvents:    []string{"Warning/CreateAttemptsExhausted"},
		},
		"BelowLimit_NotFound": {
			args: args{
				mg: serviceCredentialBinding("key",
					withExternalName(guid),
					withServiceInstanceID(serviceInstanceGUID),
					withCreateAttempts(DefaultMaxCreateAttempts-1),
				),
			},
			want: want{
				obs: managed.ExternalObservation{ResourceExists: false},
			},
			service:    cfNotFound,
			keyRotator: noKeyRotator,
		},
		"Trip_UpdateFails": {
			args: args{
				mg: serviceCredentialBinding("key",
					withExternalName(guid),
					withServiceInstanceID(serviceInstanceGUID),
					withCreateAttempts(DefaultMaxCreateAttempts),
				),
			},
			want: want{
				obs: managed.ExternalObservation{},
				err: fmt.Errorf("%s: %w", errUpdateCR, errCFClientError),
			},
			service:       cfNotFound,
			keyRotator:    noKeyRotator,
			updateErr:     errCFClientError,
			wantKubeCalls: []string{"Update"},
		},
		"Trip_Retirement": {
			// A forced rotation whose replacement keeps failing.
			args: args{
				mg: serviceCredentialBinding("key",
					withExternalName(guid),
					withServiceInstanceID(serviceInstanceGUID),
					withCreateAttempts(DefaultMaxCreateAttempts),
					withRetiredKeys(guid),
				),
			},
			want: want{
				obs:     managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
				tripped: true,
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				m.On("Get", mock.Anything, guid).Return(cfSucceeded(), nil)
				return m
			},
			keyRotator: func() *fake.MockKeyRotator {
				m := &fake.MockKeyRotator{}
				m.On("RetireBinding", mock.Anything, mock.Anything).Return(true)
				return m
			},
			wantKubeCalls: []string{"StatusUpdate", "Update"},
			wantEvents:    []string{"Warning/CreateAttemptsExhausted"},
		},
		"Retirement_BelowLimit": {
			args: args{
				mg: serviceCredentialBinding("key",
					withExternalName(guid),
					withServiceInstanceID(serviceInstanceGUID),
				),
			},
			want: want{
				obs: managed.ExternalObservation{ResourceExists: false},
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				m.On("Get", mock.Anything, guid).Return(cfSucceeded(), nil)
				return m
			},
			keyRotator: func() *fake.MockKeyRotator {
				m := &fake.MockKeyRotator{}
				m.On("RetireBinding", mock.Anything, mock.Anything).Return(true)
				return m
			},
			wantKubeCalls: []string{"StatusUpdate"},
		},
		"LimitHonoured_Low": {
			args: args{
				mg: serviceCredentialBinding("key",
					withExternalName(guid),
					withServiceInstanceID(serviceInstanceGUID),
					withCreateAttempts(2),
				),
			},
			limit: 2,
			want: want{
				obs:     managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
				tripped: true,
			},
			service:       cfNotFound,
			keyRotator:    noKeyRotator,
			wantKubeCalls: []string{"Update"},
			wantEvents:    []string{"Warning/CreateAttemptsExhausted"},
		},
		"LimitHonoured_High": {
			args: args{
				mg: serviceCredentialBinding("key",
					withExternalName(guid),
					withServiceInstanceID(serviceInstanceGUID),
					withCreateAttempts(5),
				),
			},
			limit: 10,
			want: want{
				obs: managed.ExternalObservation{ResourceExists: false},
			},
			service:    cfNotFound,
			keyRotator: noKeyRotator,
		},
		"Adoption_ResetsCounter": {
			args: args{
				mg: serviceCredentialBinding("key",
					withExternalName("my-key-name"),
					withServiceInstanceID(serviceInstanceGUID),
					withCreateAttempts(DefaultMaxCreateAttempts),
				),
			},
			want: want{
				mg: serviceCredentialBinding("key",
					withExternalName(guid),
					withServiceInstanceID(serviceInstanceGUID),
					withObservation(guid, &v1alpha1.LastOperation{
						Type:        "create",
						State:       "succeeded",
						Description: "create succeeded",
						CreatedAt:   "0001-01-01 00:00:00 +0000 UTC",
						UpdatedAt:   "",
					}),
				),
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
				err: nil,
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				// First call with invalid name fails
				m.On("Get", mock.Anything, "my-key-name").Return(
					fake.ServiceCredentialBindingNil,
					fake.ErrNoResultReturned,
				)
				// Second call via search succeeds
				m.On("Single", mock.Anything, mock.Anything).Return(
					&fake.NewServiceCredentialBinding("key").SetGUID(guid).SetName(name).SetLastOperation(v1alpha1.LastOperationCreate, v1alpha1.LastOperationSucceeded).ServiceCredentialBinding,
					nil,
				)
				return m
			},
			keyRotator: func() *fake.MockKeyRotator {
				m := &fake.MockKeyRotator{}
				m.On("RetireBinding", mock.Anything, mock.Anything).Return(false)
				return m
			},
			observationStateHandler: func() *MockObservationStateHandler {
				m := &MockObservationStateHandler{}
				m.On("HandleObservationState", mock.Anything, mock.Anything, mock.Anything).Return(
					managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
					nil,
				)
				return m
			},
			wantKubeCalls: []string{"Update"},
		},
		"InvalidGUIDFormat_FallsBackToSpecSearch": {
			args: args{
				mg: serviceCredentialBinding("key", withExternalName("not-a-uuid"), withServiceInstanceID(serviceInstanceGUID)),
			},
			want: want{
				mg:  serviceCredentialBinding("key", withExternalName("not-a-uuid"), withServiceInstanceID(serviceInstanceGUID)),
				obs: managed.ExternalObservation{ResourceExists: false},
				err: nil,
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				m.On("Single", mock.Anything, mock.Anything).Return(
					fake.ServiceCredentialBindingNil,
					fake.ErrNoResultReturned,
				)
				return m
			},
			keyRotator: func() *fake.MockKeyRotator {
				m := &fake.MockKeyRotator{}
				return m
			},
		},
		"Adoption_ExternalNameUpdatedToGUID": {
			args: args{
				mg: serviceCredentialBinding("key", withExternalName("my-key-name"), withServiceInstanceID(serviceInstanceGUID)),
			},
			want: want{
				mg: serviceCredentialBinding("key",
					withExternalName(guid),
					withServiceInstanceID(serviceInstanceGUID),
					withObservation(guid, &v1alpha1.LastOperation{
						Type:        "create",
						State:       "succeeded",
						Description: "create succeeded",
						CreatedAt:   "0001-01-01 00:00:00 +0000 UTC",
						UpdatedAt:   "",
					}),
					withObservedAnnotations(map[string]*string{
						"crossplane-kind": ptr.To("servicecredentialbinding.cloudfoundry.crossplane.io"),
						"crossplane-name": ptr.To("my-service-credential-binding"),
					}),
				),
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
				err: nil,
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				m.On("Get", mock.Anything, "my-key-name").Return(
					fake.ServiceCredentialBindingNil,
					fake.ErrNoResultReturned,
				)
				m.On("Single", mock.Anything, mock.Anything).Return(
					cfSucceeded(),
					nil,
				)
				return m
			},
			kube: &test.MockClient{
				MockUpdate: test.NewMockUpdateFn(nil),
			},
			keyRotator: func() *fake.MockKeyRotator {
				m := &fake.MockKeyRotator{}
				m.On("RetireBinding", mock.Anything, mock.Anything).Return(false)
				return m
			},
			observationStateHandler: func() *MockObservationStateHandler {
				m := &MockObservationStateHandler{}
				m.On("HandleObservationState", cfSucceeded(), mock.Anything, mock.Anything).Return(
					managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
					nil,
				)
				return m
			},
		},
		"Delete_Found_SkipsRetirement": {
			// Found during deletion: must stay "exists" so Delete() runs.
			args: args{
				mg: serviceCredentialBinding("key",
					withExternalName(guid),
					withServiceInstanceID(serviceInstanceGUID),
					withDeletionTimestamp(),
				),
			},
			want: want{
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				m.On("Get", mock.Anything, guid).Return(cfSucceeded(), nil)
				return m
			},
			keyRotator: func() *fake.MockKeyRotator {
				m := &fake.MockKeyRotator{}
				m.On("RetireBinding", mock.Anything, mock.Anything).Return(true)
				return m
			},
			observationStateHandler: func() *MockObservationStateHandler {
				m := &MockObservationStateHandler{}
				m.On("HandleObservationState", mock.Anything, mock.Anything, mock.Anything).Return(
					managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, nil,
				)
				return m
			},
			wantNoRetire: true,
		},
		"Delete_NotFound_DeletesRetiredKeys": {
			args: args{
				mg: serviceCredentialBinding("key",
					withExternalName(guid),
					withServiceInstanceID(serviceInstanceGUID),
					withDeletionTimestamp(),
					withRetiredKeys("11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222"),
				),
			},
			want: want{
				obs: managed.ExternalObservation{ResourceExists: false},
			},
			service: cfNotFound,
			keyRotator: func() *fake.MockKeyRotator {
				m := &fake.MockKeyRotator{}
				m.On("DeleteRetiredKeys", mock.Anything, mock.Anything).Return(nil)
				return m
			},
			wantDeleteRetiredKeys: 1,
		},
		"Delete_NotFound_RetiredKeyCleanupFails": {
			args: args{
				mg: serviceCredentialBinding("key",
					withExternalName(guid),
					withServiceInstanceID(serviceInstanceGUID),
					withDeletionTimestamp(),
					withRetiredKeys("11111111-1111-1111-1111-111111111111"),
				),
			},
			want: want{
				obs: managed.ExternalObservation{},
				err: fmt.Errorf(errDeleteRetiredKeys, errCFClientError),
			},
			service: cfNotFound,
			keyRotator: func() *fake.MockKeyRotator {
				m := &fake.MockKeyRotator{}
				m.On("DeleteRetiredKeys", mock.Anything, mock.Anything).Return(errCFClientError)
				return m
			},
			wantDeleteRetiredKeys: 1,
		},
		"Delete_NotFound_OverLimitDoesNotTrip": {
			args: args{mg: deletingOverLimit.DeepCopy()},
			want: want{
				mg:  deletingOverLimit.DeepCopy(),
				obs: managed.ExternalObservation{ResourceExists: false},
			},
			service: cfNotFound,
			keyRotator: func() *fake.MockKeyRotator {
				m := &fake.MockKeyRotator{}
				m.On("DeleteRetiredKeys", mock.Anything, mock.Anything).Return(nil)
				return m
			},
			wantDeleteRetiredKeys: 1,
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			t.Logf("Testing: %s", t.Name())
			var obsHandler ObservationStateHandler
			if tc.observationStateHandler != nil {
				obsHandler = tc.observationStateHandler()
			}
			var kubeCalls []string
			// updated holds the annotations the last kube Update received.
			var updated map[string]string
			kubeClient := tc.kube
			if kubeClient == nil {
				kubeClient = &test.MockClient{
					MockUpdate: func(_ context.Context, obj k8s.Object, _ ...k8s.UpdateOption) error {
						kubeCalls = append(kubeCalls, "Update")
						updated = maps.Clone(obj.GetAnnotations())
						return tc.updateErr
					},
					MockStatusUpdate: func(context.Context, k8s.Object, ...k8s.SubResourceUpdateOption) error {
						kubeCalls = append(kubeCalls, "StatusUpdate")
						return nil
					},
				}
			}
			limit := tc.limit
			if limit == 0 {
				limit = DefaultMaxCreateAttempts
			}
			kr := tc.keyRotator()
			rec := &fakeRecorder{}
			c := &external{
				kube:                    kubeClient,
				scbClient:               tc.service(),
				keyRotator:              kr,
				observationStateHandler: obsHandler,
				maxCreateAttempts:       limit,
				recorder:                rec,
			}
			obs, err := c.Observe(context.Background(), tc.args.mg)

			if tc.want.err != nil && err != nil {
				// the case where our mock server returns error.
				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("Observe(...): want error string != got error string:\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(tc.want.err, err); diff != "" {
					t.Errorf("Observe(...): want error != got error:\n%s", diff)
				}
			}
			if diff := cmp.Diff(tc.want.obs, obs); diff != "" {
				t.Errorf("Observe(...): -want, +got:\n%s", diff)
			}
			if tc.want.mg != nil {
				// Ignore UpdatedAt timestamp in LastOperation as it's set to time.Now() by the fake
				ignoreUpdateTime := cmp.FilterPath(func(p cmp.Path) bool {
					return p.String() == "Status.AtProvider.LastOperation.UpdatedAt"
				}, cmp.Ignore())
				if diff := cmp.Diff(tc.want.mg, tc.args.mg, ignoreUpdateTime); diff != "" {
					t.Errorf("Observe(...): -want, +got:\n%s", diff)
				}
			}
			if cr, ok := tc.args.mg.(*v1alpha1.ServiceCredentialBinding); ok {
				readyMsg := cr.GetCondition(xpv1.TypeReady).Message
				switch {
				case tc.want.err != nil:
					// A failed Observe must not claim the resource is paused.
					if strings.Contains(readyMsg, MaxRetryExceededKey) {
						t.Errorf("Observe(...): Ready message %q names %s although Observe failed", readyMsg, MaxRetryExceededKey)
					}
				case tc.want.tripped:
					// Check what was persisted, not the in-memory object.
					marker, marked := updated[MaxRetryExceededKey]
					if !marked {
						t.Errorf("Observe(...): kube Update did not receive the %s marker", MaxRetryExceededKey)
					} else if _, err := time.Parse(time.RFC3339, marker); err != nil {
						t.Errorf("Observe(...): marker value %q is not RFC3339: %v", marker, err)
					}
					if _, counted := updated[createAttemptsAnnotation]; counted {
						t.Errorf("Observe(...): kube Update must not receive the counter annotation on trip")
					}
					if !strings.Contains(readyMsg, MaxRetryExceededKey) {
						t.Errorf("Observe(...): Ready message %q does not name %s", readyMsg, MaxRetryExceededKey)
					}
				default:
					if _, marked := cr.GetAnnotations()[MaxRetryExceededKey]; marked {
						t.Errorf("Observe(...): want no %s marker, got one", MaxRetryExceededKey)
					}
				}
			}
			if tc.kube == nil {
				if diff := cmp.Diff(tc.wantKubeCalls, kubeCalls, cmpopts.EquateEmpty()); diff != "" {
					t.Errorf("Observe(...): kube writes -want, +got:\n%s", diff)
				}
			}
			gotEvents := make([]string, 0, len(rec.events))
			for _, e := range rec.events {
				gotEvents = append(gotEvents, string(e.Type)+"/"+string(e.Reason))
			}
			if diff := cmp.Diff(tc.wantEvents, gotEvents, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("Observe(...): events -want, +got:\n%s", diff)
			}
			kr.AssertNumberOfCalls(t, "DeleteRetiredKeys", tc.wantDeleteRetiredKeys)
			if tc.wantNoRetire {
				kr.AssertNotCalled(t, "RetireBinding", mock.Anything, mock.Anything)
			}
		})
	}
}

func TestUpdate(t *testing.T) {
	type service func() *fake.MockServiceCredentialBinding
	type keyRotator func() *fake.MockKeyRotator
	type args struct {
		mg resource.Managed
	}

	type want struct {
		mg  resource.Managed
		obs managed.ExternalUpdate
		err error
	}

	retiredKey1 := &v1alpha1.SCBResource{
		GUID:      "retired-key-1",
		CreatedAt: &metav1.Time{Time: time.Now().Add(-2 * time.Hour)},
	}

	mgWithRetiredKeys := serviceCredentialBinding("key", withServiceInstanceID(serviceInstanceGUID), withExternalName(guid), withStatus(guid))
	mgWithRetiredKeys.Status.AtProvider.RetiredKeys = []*v1alpha1.SCBResource{retiredKey1}

	cases := map[string]struct {
		args       args
		want       want
		service    service
		keyRotator keyRotator
	}{
		"Successful": {
			args: args{
				mg: serviceCredentialBinding("key", withServiceInstanceID(serviceInstanceGUID), withExternalName(guid)),
			},
			want: want{
				mg:  serviceCredentialBinding("key", withServiceInstanceID(serviceInstanceGUID), withExternalName(guid)),
				obs: managed.ExternalUpdate{},
				err: nil,
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				m.On("Update", mock.Anything, guid, mock.Anything).Return(
					&fake.NewServiceCredentialBinding("key").SetName(name).SetGUID(guid).ServiceCredentialBinding,
					nil,
				)
				return m
			},
			keyRotator: func() *fake.MockKeyRotator {
				m := &fake.MockKeyRotator{}
				return m
			},
		},
		"EmptyExternalName": {
			args: args{
				mg: serviceCredentialBinding("key", withServiceInstanceID(serviceInstanceGUID)),
			},
			want: want{
				mg:  serviceCredentialBinding("key", withServiceInstanceID(serviceInstanceGUID)),
				obs: managed.ExternalUpdate{},
				err: nil,
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				return m
			},
			keyRotator: func() *fake.MockKeyRotator {
				m := &fake.MockKeyRotator{}
				return m
			},
		},
		"UpdateFailed": {
			args: args{
				mg: serviceCredentialBinding("key", withServiceInstanceID(serviceInstanceGUID), withExternalName(guid)),
			},
			want: want{
				mg:  serviceCredentialBinding("key", withServiceInstanceID(serviceInstanceGUID), withExternalName(guid)),
				obs: managed.ExternalUpdate{},
				err: fmt.Errorf(errUpdate, errCFClientError),
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				m.On("Update", mock.Anything, guid, mock.Anything).Return(
					(*cfresource.ServiceCredentialBinding)(nil),
					errCFClientError,
				)
				return m
			},
			keyRotator: func() *fake.MockKeyRotator {
				m := &fake.MockKeyRotator{}
				return m
			},
		},
		"WithRetiredKeysSuccessful": {
			args: args{
				mg: mgWithRetiredKeys.DeepCopy(),
			},
			want: want{
				mg:  mgWithRetiredKeys.DeepCopy(),
				obs: managed.ExternalUpdate{},
				err: nil,
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				m.On("Update", mock.Anything, guid, mock.Anything).Return(
					&fake.NewServiceCredentialBinding("key").SetName(name).SetGUID(guid).ServiceCredentialBinding,
					nil,
				)
				return m
			},
			keyRotator: func() *fake.MockKeyRotator {
				m := &fake.MockKeyRotator{}
				m.On("DeleteExpiredKeys", mock.Anything, mgWithRetiredKeys.DeepCopy()).Return(
					[]*v1alpha1.SCBResource{}, // All keys deleted
					nil,
				)
				return m
			},
		},
		"DeleteExpiredKeysFailed": {
			args: args{
				mg: mgWithRetiredKeys.DeepCopy(),
			},
			want: want{
				mg:  mgWithRetiredKeys.DeepCopy(),
				obs: managed.ExternalUpdate{},
				err: fmt.Errorf(errDeleteExpiredKeys, errCFClientError),
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				m.On("Update", mock.Anything, guid, mock.Anything).Return(
					&fake.NewServiceCredentialBinding("key").SetName(name).SetGUID(guid).ServiceCredentialBinding,
					nil,
				)
				return m
			},
			keyRotator: func() *fake.MockKeyRotator {
				m := &fake.MockKeyRotator{}
				m.On("DeleteExpiredKeys", mock.Anything, mgWithRetiredKeys.DeepCopy()).Return(
					[]*v1alpha1.SCBResource{},
					errCFClientError,
				)
				return m
			},
		},
		"InvalidUUID_SkipsUpdate": {
			args: args{
				mg: serviceCredentialBinding("key", withServiceInstanceID(serviceInstanceGUID), withExternalName("my-key-name")),
			},
			want: want{
				mg:  serviceCredentialBinding("key", withServiceInstanceID(serviceInstanceGUID), withExternalName("my-key-name")),
				obs: managed.ExternalUpdate{},
				err: nil,
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				// No Update mock registered — if Update is called, testify will panic
				return m
			},
			keyRotator: func() *fake.MockKeyRotator {
				m := &fake.MockKeyRotator{}
				return m
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			t.Logf("Testing: %s", t.Name())
			c := &external{
				kube: &test.MockClient{
					MockUpdate:       test.NewMockUpdateFn(nil),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				scbClient:  tc.service(),
				keyRotator: tc.keyRotator(),
			}
			obs, err := c.Update(context.Background(), tc.args.mg)

			if tc.want.err != nil && err != nil {
				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("Update(...): want error string != got error string:\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(tc.want.err, err); diff != "" {
					t.Errorf("Update(...): want error != got error:\n%s", diff)
				}
			}
			if diff := cmp.Diff(tc.want.obs, obs); diff != "" {
				t.Errorf("Update(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestConnector(t *testing.T) {
	errKube := errors.New("boom")
	// Every Get fails, so a nil error proves Connect never built a CF client.
	failingKube := func() k8s.Client {
		return &test.MockClient{MockGet: test.NewMockGetFn(errKube)}
	}
	errClientBuild := errors.New("cannot create a client for Cloud Foundry: cannot config cloudfoundry client: cannot get referenced ProviderConfig: boom")

	type want struct {
		paused  bool
		tracked int
		err     error
	}

	cases := map[string]struct {
		mg   resource.Managed
		kube k8s.Client
		want want
	}{
		"WrongCRType": {
			mg:   &v1alpha1.App{},
			kube: &test.MockClient{},
			want: want{err: errors.New(errWrongCRType)},
		},
		"Paused": {
			mg:   serviceCredentialBinding("key", withProviderConfigRef("default"), withMaxRetryExceeded()),
			kube: failingKube(),
			want: want{paused: true, tracked: 1},
		},
		"PausedButDeleting": {
			mg:   serviceCredentialBinding("key", withProviderConfigRef("default"), withMaxRetryExceeded(), withDeletionTimestamp()),
			kube: failingKube(),
			want: want{tracked: 1, err: errClientBuild},
		},
		"NotPaused": {
			mg:   serviceCredentialBinding("key", withProviderConfigRef("default")),
			kube: failingKube(),
			want: want{tracked: 1, err: errClientBuild},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			tracked := 0
			c := &connector{
				kube: tc.kube,
				usage: resource.LegacyTrackerFn(func(context.Context, resource.LegacyManaged) error {
					tracked++
					return nil
				}),
				maxCreateAttempts: DefaultMaxCreateAttempts,
			}

			client, err := c.Connect(context.Background(), tc.mg)

			if tc.want.err != nil && err != nil {
				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("Connect(...): want error string != got error string:\n%s", diff)
				}
			} else if diff := cmp.Diff(tc.want.err, err); diff != "" {
				t.Errorf("Connect(...): want error != got error:\n%s", diff)
			}
			if tc.want.err != nil && client != nil {
				t.Errorf("Connect(...): expected nil client on error, got %T", client)
			}
			if _, paused := client.(*pausedExternal); paused != tc.want.paused {
				t.Errorf("Connect(...): want paused client %t, got %T", tc.want.paused, client)
			}
			if tracked != tc.want.tracked {
				t.Errorf("Connect(...): want %d usage tracking calls, got %d", tc.want.tracked, tracked)
			}
		})
	}
}

func TestPausedExternal(t *testing.T) {
	ctx := context.Background()
	p := &pausedExternal{maxCreateAttempts: 3}
	cr := serviceCredentialBinding("key", withMaxRetryExceeded())

	obs, err := p.Observe(ctx, cr)
	if err != nil {
		t.Fatalf("Observe(...): unexpected error: %v", err)
	}
	if diff := cmp.Diff(managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, obs); diff != "" {
		t.Errorf("Observe(...): -want, +got:\n%s", diff)
	}
	ready := cr.GetCondition(xpv1.TypeReady)
	if ready.Reason != xpv1.ReasonUnavailable {
		t.Errorf("Observe(...): want Ready reason %q, got %q", xpv1.ReasonUnavailable, ready.Reason)
	}
	for _, s := range []string{MaxRetryExceededKey, "after 3 attempts", "with 3 fresh attempts"} {
		if !strings.Contains(ready.Message, s) {
			t.Errorf("Observe(...): Ready message %q does not contain %q", ready.Message, s)
		}
	}

	// Unreachable via the reconciler, but must still refuse to act.
	for name, call := range map[string]func() error{
		"Create": func() error { _, err := p.Create(ctx, cr); return err },
		"Update": func() error { _, err := p.Update(ctx, cr); return err },
		"Delete": func() error { _, err := p.Delete(ctx, cr); return err },
	} {
		if err := call(); err == nil || err.Error() != errPaused {
			t.Errorf("%s(...): want error %q, got %v", name, errPaused, err)
		}
	}
}

func TestHandleObservationState(t *testing.T) {
	type args struct {
		serviceBinding *cfresource.ServiceCredentialBinding
		ctx            context.Context
		cr             *v1alpha1.ServiceCredentialBinding
	}

	type want struct {
		obs managed.ExternalObservation
		err error
	}

	ctx := context.Background()
	cr := serviceCredentialBinding("key", withExternalName(guid), withServiceInstanceID(serviceInstanceGUID), withDefaultMetadata())

	scbCreate := func(lastOperation string) *cfresource.ServiceCredentialBinding {
		return &fake.NewServiceCredentialBinding("key").SetName(name).SetGUID(guid).SetServiceInstanceRef(serviceInstanceGUID).SetLastOperation(v1alpha1.LastOperationCreate, lastOperation).SetAnnotations(map[string]*string{"crossplane-kind": ptr.To("servicecredentialbinding.cloudfoundry.crossplane.io"), "crossplane-name": ptr.To("my-service-credential-binding")}).ServiceCredentialBinding
	}

	cases := map[string]struct {
		args args
		want want
		kube k8s.Client
	}{
		"LastOperationInitial": {
			args: args{
				serviceBinding: scbCreate(v1alpha1.LastOperationInitial),
				ctx:            ctx,
				cr:             cr.DeepCopy(),
			},
			want: want{
				obs: managed.ExternalObservation{
					ResourceExists:   true,
					ResourceUpToDate: true,
				},
				err: nil,
			},
		},
		"LastOperationInProgress": {
			args: args{
				serviceBinding: scbCreate(v1alpha1.LastOperationInProgress),
				ctx:            ctx,
				cr:             cr.DeepCopy(),
			},
			want: want{
				obs: managed.ExternalObservation{
					ResourceExists:   true,
					ResourceUpToDate: true,
				},
				err: nil,
			},
		},
		"LastOperationCreateFailed": {
			args: args{
				serviceBinding: scbCreate(v1alpha1.LastOperationFailed),
				ctx:            ctx,
				cr:             cr.DeepCopy(),
			},
			want: want{
				obs: managed.ExternalObservation{
					ResourceExists:   true, // circuit breaker handles retry gating
					ResourceUpToDate: true,
				},
				err: nil,
			},
		},
		"LastOperationUpdateFailed": {
			args: args{
				serviceBinding: &fake.NewServiceCredentialBinding("key").SetName(name).SetGUID(guid).SetServiceInstanceRef(serviceInstanceGUID).SetLastOperation(v1alpha1.LastOperationUpdate, v1alpha1.LastOperationFailed).ServiceCredentialBinding,
				ctx:            ctx,
				cr:             cr.DeepCopy(),
			},
			want: want{
				obs: managed.ExternalObservation{
					ResourceExists:   true,  // Update failed, but resource still exists
					ResourceUpToDate: false, // Update failed, so not up to date
				},
				err: nil,
			},
		},
		"LastOperationSucceeded": {
			args: args{
				serviceBinding: scbCreate(v1alpha1.LastOperationSucceeded),
				ctx:            ctx,
				cr:             cr.DeepCopy(),
			},
			want: want{
				obs: managed.ExternalObservation{
					ResourceExists:    true,
					ResourceUpToDate:  true, // Assuming IsUpToDate returns true and no expired keys
					ConnectionDetails: managed.ConnectionDetails{},
				},
				err: nil,
			},
		},
		"LegacyDefaultLabelsNotUpToDate": {
			args: args{
				serviceBinding: func() *cfresource.ServiceCredentialBinding {
					legacy := map[string]*string{
						"crossplane-kind": ptr.To("servicecredentialbinding.cloudfoundry.crossplane.io"),
						"crossplane-name": ptr.To("my-service-credential-binding"),
					}
					return &fake.NewServiceCredentialBinding("key").SetName(name).SetGUID(guid).SetServiceInstanceRef(serviceInstanceGUID).SetLastOperation(v1alpha1.LastOperationCreate, v1alpha1.LastOperationSucceeded).SetLabels(legacy).SetAnnotations(legacy).ServiceCredentialBinding
				}(),
				ctx: ctx,
				cr:  cr.DeepCopy(),
			},
			want: want{
				obs: managed.ExternalObservation{
					ResourceExists:    true,
					ResourceUpToDate:  false, // legacy crossplane-* labels must be removed
					ConnectionDetails: managed.ConnectionDetails{},
				},
				err: nil,
			},
		},
		"UnknownState": {
			args: args{
				serviceBinding: &cfresource.ServiceCredentialBinding{
					LastOperation: cfresource.LastOperation{
						State: "unknown-state",
						Type:  v1alpha1.LastOperationCreate,
					},
				},
				ctx: ctx,
				cr:  cr.DeepCopy(),
			},
			want: want{
				obs: managed.ExternalObservation{},
				err: errors.New(errUnknownState),
			},
		},
		"LastOperationSucceeded_KubeUpdateFails": {
			args: args{
				serviceBinding: scbCreate(v1alpha1.LastOperationSucceeded),
				ctx:            ctx,
				cr: serviceCredentialBinding("key",
					withExternalName(guid),
					withServiceInstanceID(serviceInstanceGUID),
					withCreateAttempts(3), // needs non-zero counter to trigger the update
				),
			},
			kube: &test.MockClient{
				MockUpdate: test.NewMockUpdateFn(errCFClientError),
			},
			want: want{
				obs: managed.ExternalObservation{},
				err: fmt.Errorf("cannot persist create attempt reset: %w", errCFClientError),
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			t.Logf("Testing: %s", t.Name())
			kube := tc.kube
			if kube == nil {
				kube = &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(nil),
				}
			}

			// Create external with mocked dependencies
			c := &external{
				kube:       kube,
				scbClient:  &fake.MockServiceCredentialBinding{},
				keyRotator: &fake.MockKeyRotator{},
			}

			// Set up mocks for the successful case
			if tc.args.serviceBinding.LastOperation.State == v1alpha1.LastOperationSucceeded {
				mockSCB := c.scbClient.(*fake.MockServiceCredentialBinding)
				mockSCB.On("GetDetails", mock.Anything, guid).Return(
					fake.NewServiceCredentialBindingDetails(guid),
					nil,
				)

				mockKeyRotator := c.keyRotator.(*fake.MockKeyRotator)
				mockKeyRotator.On("HasExpiredKeys", tc.args.cr).Return(false)
			}

			obs, err := c.HandleObservationState(tc.args.serviceBinding, tc.args.ctx, tc.args.cr)

			if tc.want.err != nil && err != nil {
				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("HandleObservationState(...): want error string != got error string:\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(tc.want.err, err); diff != "" {
					t.Errorf("HandleObservationState(...): want error != got error:\n%s", diff)
				}
			}
			if diff := cmp.Diff(tc.want.obs, obs); diff != "" {
				t.Errorf("HandleObservationState(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestCreate(t *testing.T) {
	type service func() *fake.MockServiceCredentialBinding
	type args struct {
		mg resource.Managed
	}

	type want struct {
		mg  resource.Managed
		obs managed.ExternalCreation
		err error
	}

	scbKey := func() *cfresource.ServiceCredentialBinding {
		return &fake.NewServiceCredentialBinding("key").SetName(name).SetGUID(guid).SetServiceInstanceRef(serviceInstanceGUID).ServiceCredentialBinding
	}
	scbApp := func() *cfresource.ServiceCredentialBinding {
		return &fake.NewServiceCredentialBinding("app").SetName(name).SetGUID(guid).SetServiceInstanceRef(serviceInstanceGUID).ServiceCredentialBinding
	}

	cases := map[string]struct {
		args       args
		want       want
		service    service
		kube       k8s.Client
		keyRotator servicecredentialbinding.KeyRotator
	}{
		"Successful": {
			args: args{
				mg: serviceCredentialBinding("key", withServiceInstanceID(serviceInstanceGUID)),
			},
			want: want{
				mg: serviceCredentialBinding(
					"key",
					withExternalName(guid),
					withServiceInstanceID(serviceInstanceGUID),
					withCreateAttempts(1),
				),
				obs: managed.ExternalCreation{},
				err: nil,
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				m.On("Create", mock.Anything, mock.Anything).Return(
					guid,
					scbKey(),
					nil,
				)
				m.On("Single", mock.Anything, mock.Anything).Return(
					scbKey(),
					nil,
				)
				m.On("PollComplete", mock.Anything, mock.Anything, mock.Anything).Return(nil)
				return m
			},
		},
		"Should fail if Service Instance is missing": {
			args: args{
				mg: serviceCredentialBinding("key"),
			},
			want: want{
				mg:  serviceCredentialBinding("key", withCreateAttempts(1)),
				obs: managed.ExternalCreation{},
				err: fmt.Errorf(errCreate, errServiceInstanceMissing),
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}

				m.On("Create", mock.Anything, mock.Anything).Return(
					guid,
					scbKey(),
					nil,
				)

				m.On("Single", mock.Anything, mock.Anything).Return(
					scbKey(),
					nil,
				)
				m.On("PollComplete", mock.Anything, mock.Anything, mock.Anything).Return(nil)

				return m
			},
		},
		"Should fail if App is missing for type app": {
			args: args{
				mg: serviceCredentialBinding("app", withServiceInstanceID(serviceInstanceGUID)),
			},
			want: want{
				mg: serviceCredentialBinding("app", withServiceInstanceID(serviceInstanceGUID), withCreateAttempts(1)),

				obs: managed.ExternalCreation{},
				err: fmt.Errorf(errCreate, errAppMissing),
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}

				m.On("Create", mock.Anything, mock.Anything).Return(
					guid,
					scbApp(),
					nil,
				)

				m.On("Single", mock.Anything, mock.Anything).Return(
					scbApp(),
					nil,
				)
				m.On("PollComplete", mock.Anything, mock.Anything, mock.Anything).Return(nil)

				return m
			},
		},
		"PollError": {
			args: args{
				mg: serviceCredentialBinding("key", withServiceInstanceID(serviceInstanceGUID)),
			},
			want: want{
				mg: serviceCredentialBinding(
					"key",
					withServiceInstanceID(serviceInstanceGUID),
					withCreateAttempts(1),
					withExternalName(guid),
				),
				obs: managed.ExternalCreation{},
				err: fmt.Errorf(errCreate, errCFClientError),
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}

				m.On("Create", mock.Anything, mock.Anything).Return(
					guid,
					scbKey(),
					nil,
				)

				m.On("Single", mock.Anything, mock.Anything).Return(
					scbKey(),
					nil,
				)
				m.On("PollComplete", mock.Anything, mock.Anything, mock.Anything).Return(errCFClientError)

				return m
			},
		},
		"PollErrorBindingNotFound": {
			args: args{
				mg: serviceCredentialBinding("key", withServiceInstanceID(serviceInstanceGUID)),
			},
			want: want{
				mg: serviceCredentialBinding(
					"key",
					withServiceInstanceID(serviceInstanceGUID),
					withCreateAttempts(1),
				),
				obs: managed.ExternalCreation{},
				err: fmt.Errorf(errCreate, errCFClientError),
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}

				m.On("Create", mock.Anything, mock.Anything).Return(
					guid,
					scbKey(),
					nil,
				)

				m.On("Single", mock.Anything, mock.Anything).Return(
					fake.ServiceCredentialBindingNil,
					fake.ErrNoResultReturned,
				)
				m.On("PollComplete", mock.Anything, mock.Anything, mock.Anything).Return(errCFClientError)

				return m
			},
		},
		"AlreadyExist": {
			args: args{
				mg: serviceCredentialBinding("key", withServiceInstanceID(serviceInstanceGUID)),
			},
			want: want{
				mg: serviceCredentialBinding(
					"key",
					withServiceInstanceID(serviceInstanceGUID),
					withCreateAttempts(1),
					withExternalName(guid),
				),

				obs: managed.ExternalCreation{},
				err: fmt.Errorf(errCreate, errCFClientError),
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				m.On("Create", mock.Anything, mock.Anything).Return(
					guid,
					scbKey(),
					errCFClientError,
				)
				m.On("Single", mock.Anything, mock.Anything).Return(
					scbKey(),
					nil,
				)
				m.On("Get", mock.Anything, mock.Anything).Return(
					scbKey(),
					nil,
				)
				m.On("PollComplete", mock.Anything, mock.Anything, mock.Anything).Return(nil)

				return m
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			t.Logf("Testing: %s", t.Name())
			c := &external{
				kube: &test.MockClient{
					MockUpdate:       test.NewMockUpdateFn(nil),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				scbClient: tc.service(),
			}
			obs, err := c.Create(context.Background(), tc.args.mg)

			if tc.want.err != nil && err != nil {
				// the case where our mock server returns error.
				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("Observe(...): want error string != got error string:\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(tc.want.err, err); diff != "" {
					t.Errorf("Observe(...): want error != got error:\n%s", diff)
				}
			}
			if diff := cmp.Diff(tc.want.obs, obs); diff != "" {
				t.Errorf("Observe(...): -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.mg, tc.args.mg); diff != "" {
				t.Errorf("Observe(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	type service func() *fake.MockServiceCredentialBinding
	type keyRotator func() *fake.MockKeyRotator
	type args struct {
		mg resource.Managed
	}

	type want struct {
		mg  resource.Managed
		err error
	}

	mgArg := serviceCredentialBinding("key", withServiceInstanceID(serviceInstanceGUID), withExternalName(guid), withStatus(guid))
	mgWant := serviceCredentialBinding("key", withServiceInstanceID(serviceInstanceGUID), withExternalName(guid), withStatus(guid), withConditions(xpv1.Deleting()))

	cases := map[string]struct {
		args       args
		want       want
		service    service
		keyRotator keyRotator
	}{
		"Successful": {
			args: args{
				mg: mgArg.DeepCopy(),
			},
			want: want{
				mg:  mgWant.DeepCopy(),
				err: nil,
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				m.On("Delete", mock.Anything, guid).Return(guid, nil)
				return m
			},
			keyRotator: func() *fake.MockKeyRotator {
				m := &fake.MockKeyRotator{}
				// The object will have Deleting condition set when DeleteRetiredKeys is called
				m.On("DeleteRetiredKeys", mock.Anything, mock.MatchedBy(func(cr *v1alpha1.ServiceCredentialBinding) bool {
					return cr.GetCondition(xpv1.TypeReady).Reason == xpv1.ReasonDeleting
				})).Return(nil)
				return m
			},
		},
		"DeleteFailed": {
			args: args{
				mg: mgArg.DeepCopy(),
			},
			want: want{
				mg:  mgWant.DeepCopy(),
				err: fmt.Errorf(errDelete, errCFClientError),
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				m.On("Delete", mock.Anything, guid).Return("", errCFClientError)
				return m
			},
			keyRotator: func() *fake.MockKeyRotator {
				m := &fake.MockKeyRotator{}
				// The object will have Deleting condition set when DeleteRetiredKeys is called
				m.On("DeleteRetiredKeys", mock.Anything, mock.MatchedBy(func(cr *v1alpha1.ServiceCredentialBinding) bool {
					return cr.GetCondition(xpv1.TypeReady).Reason == xpv1.ReasonDeleting
				})).Return(nil)
				return m
			},
		},
		"DeleteRetiredKeysFailed": {
			args: args{
				mg: mgArg.DeepCopy(),
			},
			want: want{
				mg:  mgWant.DeepCopy(), // Should have Deleting condition set even if DeleteRetiredKeys fails
				err: fmt.Errorf(errDeleteRetiredKeys, errCFClientError),
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				// Delete should not be called if DeleteRetiredKeys fails
				return m
			},
			keyRotator: func() *fake.MockKeyRotator {
				m := &fake.MockKeyRotator{}
				// The object will have Deleting condition set when DeleteRetiredKeys is called
				m.On("DeleteRetiredKeys", mock.Anything, mock.MatchedBy(func(cr *v1alpha1.ServiceCredentialBinding) bool {
					return cr.GetCondition(xpv1.TypeReady).Reason == xpv1.ReasonDeleting
				})).Return(errCFClientError)
				return m
			},
		},
		"NotFound_IgnoredOnDelete": {
			args: args{
				mg: mgArg.DeepCopy(),
			},
			want: want{
				mg:  mgWant.DeepCopy(),
				err: nil,
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				m.On("Delete", mock.Anything, guid).Return("", fake.ErrNoResultReturned)
				return m
			},
			keyRotator: func() *fake.MockKeyRotator {
				m := &fake.MockKeyRotator{}
				m.On("DeleteRetiredKeys", mock.Anything, mock.MatchedBy(func(cr *v1alpha1.ServiceCredentialBinding) bool {
					return cr.GetCondition(xpv1.TypeReady).Reason == xpv1.ReasonDeleting
				})).Return(nil)
				return m
			},
		},
		"InvalidUUID_SkipsDelete": {
			args: args{
				mg: serviceCredentialBinding("key", withServiceInstanceID(serviceInstanceGUID), withExternalName("my-key-name"), withStatus(guid)),
			},
			want: want{
				mg:  serviceCredentialBinding("key", withServiceInstanceID(serviceInstanceGUID), withExternalName("my-key-name"), withStatus(guid), withConditions(xpv1.Deleting())),
				err: nil,
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				// External-name is invalid UUID, but status has valid GUID - should use status GUID
				m.On("Delete", mock.Anything, guid).Return("", nil)
				return m
			},
			keyRotator: func() *fake.MockKeyRotator {
				m := &fake.MockKeyRotator{}
				m.On("DeleteRetiredKeys", mock.Anything, mock.MatchedBy(func(cr *v1alpha1.ServiceCredentialBinding) bool {
					return cr.GetCondition(xpv1.TypeReady).Reason == xpv1.ReasonDeleting
				})).Return(nil)
				return m
			},
		},
		"InvalidUUID_BothInvalid_SkipsDelete": {
			args: args{
				mg: serviceCredentialBinding("key", withServiceInstanceID(serviceInstanceGUID), withExternalName("my-key-name")),
			},
			want: want{
				mg:  serviceCredentialBinding("key", withServiceInstanceID(serviceInstanceGUID), withExternalName("my-key-name"), withConditions(xpv1.Deleting())),
				err: nil,
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				// No Delete mock — both external-name and status GUID are invalid, so Delete should not be called
				return m
			},
			keyRotator: func() *fake.MockKeyRotator {
				m := &fake.MockKeyRotator{}
				m.On("DeleteRetiredKeys", mock.Anything, mock.MatchedBy(func(cr *v1alpha1.ServiceCredentialBinding) bool {
					return cr.GetCondition(xpv1.TypeReady).Reason == xpv1.ReasonDeleting
				})).Return(nil)
				return m
			},
		},
		"WrongCRType": {
			args: args{
				mg: &v1alpha1.App{}, // Wrong type
			},
			want: want{
				mg:  &v1alpha1.App{},
				err: errors.New(errWrongCRType),
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				return m
			},
			keyRotator: func() *fake.MockKeyRotator {
				m := &fake.MockKeyRotator{}
				return m
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			t.Logf("Testing: %s", t.Name())
			c := &external{
				kube: &test.MockClient{
					MockUpdate:       test.NewMockUpdateFn(nil),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				scbClient:  tc.service(),
				keyRotator: tc.keyRotator(),
			}
			_, err := c.Delete(context.Background(), tc.args.mg)

			if tc.want.err != nil && err != nil {
				// the case where our mock server returns error.
				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("Delete(...): want error string != got error string:\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(tc.want.err, err); diff != "" {
					t.Errorf("Delete(...): want error != got error:\n%s", diff)
				}
			}
			if diff := cmp.Diff(tc.want.mg, tc.args.mg); diff != "" {
				t.Errorf("Delete(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func withCreateAttempts(n int) modifier {
	return func(r *v1alpha1.ServiceCredentialBinding) {
		meta.AddAnnotations(r, map[string]string{
			createAttemptsAnnotation: strconv.Itoa(n),
		})
	}
}

func withProviderConfigRef(name string) modifier {
	return func(r *v1alpha1.ServiceCredentialBinding) {
		r.Spec.ProviderConfigReference = &xpv1.Reference{Name: name}
	}
}

func withMaxRetryExceeded() modifier {
	return func(r *v1alpha1.ServiceCredentialBinding) {
		meta.AddAnnotations(r, map[string]string{MaxRetryExceededKey: "2026-09-30T00:00:00Z"})
	}
}

func withDeletionTimestamp() modifier {
	return func(r *v1alpha1.ServiceCredentialBinding) {
		now := metav1.Now()
		r.SetDeletionTimestamp(&now)
	}
}

func withRetiredKeys(guids ...string) modifier {
	return func(r *v1alpha1.ServiceCredentialBinding) {
		for _, g := range guids {
			r.Status.AtProvider.RetiredKeys = append(r.Status.AtProvider.RetiredKeys, &v1alpha1.SCBResource{GUID: g})
		}
	}
}

type fakeRecorder struct {
	events []event.Event
}

func (r *fakeRecorder) Event(_ runtime.Object, e event.Event) { r.events = append(r.events, e) }

func (r *fakeRecorder) WithAnnotations(...string) event.Recorder { return r }
