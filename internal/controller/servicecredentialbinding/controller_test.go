package servicecredentialbinding

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/mock"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/fake"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/servicecredentialbinding"
)

var (
	errBoom                   = errors.New("boom")
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
		r.ObjectMeta.Annotations[meta.AnnotationKeyExternalName] = name
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

func serviceCredentialBinding(typ string, m ...modifier) *v1alpha1.ServiceCredentialBinding {
	r := &v1alpha1.ServiceCredentialBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Finalizers:  []string{},
			Annotations: map[string]string{},
		},
		Spec: v1alpha1.ServiceCredentialBindingSpec{
			ForProvider: v1alpha1.ServiceCredentialBindingParameters{Type: typ, Name: &name, ServiceInstanceRef: &xpv1.NamespacedReference{}},
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
	}

	scb := serviceCredentialBinding("key", withExternalName(guid), withServiceInstanceID(serviceInstanceGUID))
	scbAvailable := serviceCredentialBinding(
		"key",
		withExternalName(guid),
		withStatus(guid),
		withServiceInstanceID(serviceInstanceGUID),
		withConditions(xpv1.Available()),
	)

	cfSucceeded := func() *cfresource.ServiceCredentialBinding {
		return &fake.NewServiceCredentialBinding("key").SetName(name).SetGUID(guid).SetServiceInstanceRef(serviceInstanceGUID).SetLastOperation(v1alpha1.LastOperationCreate, v1alpha1.LastOperationSucceeded).ServiceCredentialBinding
	}

	cases := map[string]struct {
		args                    args
		want                    want
		service                 service
		kube                    k8s.Client
		keyRotator              keyRotator
		observationStateHandler observationStateHandler
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
		"Boom!": {
			args: args{
				mg: scb.DeepCopy(),
			},
			want: want{
				mg:  serviceCredentialBinding("key", withExternalName(guid)),
				obs: managed.ExternalObservation{},
				err: fmt.Errorf(errGet, errBoom),
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				m.On("Get", mock.Anything, guid).Return(
					fake.ServiceCredentialBindingNil,
					errBoom,
				)
				m.On("Single").Return(
					fake.ServiceCredentialBindingNil,
					errBoom,
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
				mg:  serviceCredentialBinding("key", withExternalName(guid)),
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
				mg:  scbAvailable.DeepCopy(),
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
				mg:  scb.DeepCopy(),
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
		}}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			t.Logf("Testing: %s", t.Name())
			var obsHandler ObservationStateHandler
			if tc.observationStateHandler != nil {
				obsHandler = tc.observationStateHandler()
			}
			c := &external{
				kube: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(nil),
				},
				scbClient:               tc.service(),
				keyRotator:              tc.keyRotator(),
				observationStateHandler: obsHandler,
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
				err: fmt.Errorf(errUpdate, errBoom),
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				m.On("Update", mock.Anything, guid, mock.Anything).Return(
					(*cfresource.ServiceCredentialBinding)(nil),
					errBoom,
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
				err: fmt.Errorf(errDeleteExpiredKeys, errBoom),
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
					errBoom,
				)
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
	type args struct {
		ctx context.Context
		mg  resource.Managed
	}

	type want struct {
		client managed.ExternalClient
		err    error
	}

	cases := map[string]struct {
		args args
		want want
		kube k8s.Client
	}{
		"WrongCRType": {
			args: args{
				ctx: context.Background(),
				mg:  &v1alpha1.App{}, // Wrong type
			},
			want: want{
				client: nil,
				err:    errors.New(errWrongCRType),
			},
			kube: &test.MockClient{},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			t.Logf("Testing: %s", t.Name())

			c := &connector{
				kube: tc.kube,
				// Skip usage tracker testing for now as it requires more complex setup
			}

			client, err := c.Connect(tc.args.ctx, tc.args.mg)

			if tc.want.err != nil && err != nil {
				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("Connect(...): want error string != got error string:\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(tc.want.err, err); diff != "" {
					t.Errorf("Connect(...): want error != got error:\n%s", diff)
				}
			}
			if tc.want.client == nil && client != nil {
				t.Errorf("Connect(...): expected nil client, got non-nil")
			}
		})
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
	cr := serviceCredentialBinding("key", withExternalName(guid), withServiceInstanceID(serviceInstanceGUID))

	scbCreate := func(lastOperation string) *cfresource.ServiceCredentialBinding {
		return &fake.NewServiceCredentialBinding("key").SetName(name).SetGUID(guid).SetServiceInstanceRef(serviceInstanceGUID).SetLastOperation(v1alpha1.LastOperationCreate, lastOperation).ServiceCredentialBinding
	}

	cases := map[string]struct {
		args args
		want want
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
					ResourceExists:   false, // Create failed, so resource doesn't exist
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
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			t.Logf("Testing: %s", t.Name())

			// Create external with mocked dependencies
			c := &external{
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
				mg:  serviceCredentialBinding("key"),
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
				mg: serviceCredentialBinding("app", withServiceInstanceID(serviceInstanceGUID)),

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
				),
				obs: managed.ExternalCreation{},
				err: fmt.Errorf(errCreate, errBoom),
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
				m.On("PollComplete", mock.Anything, mock.Anything, mock.Anything).Return(errBoom)

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
				),
				obs: managed.ExternalCreation{},
				err: fmt.Errorf(errCreate, errBoom),
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				m.On("Create", mock.Anything, mock.Anything).Return(
					guid,
					scbKey(),
					errBoom,
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
				err: fmt.Errorf(errDelete, errBoom),
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				m.On("Delete", mock.Anything, guid).Return("", errBoom)
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
				err: fmt.Errorf(errDeleteRetiredKeys, errBoom),
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
				})).Return(errBoom)
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
