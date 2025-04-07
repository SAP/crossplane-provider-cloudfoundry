package servicecredentialbinding

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/test"
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
func TestObserve(t *testing.T) {
	type service func() *fake.MockServiceCredentialBinding
	type args struct {
		mg resource.Managed
	}

	type want struct {
		mg  resource.Managed
		obs managed.ExternalObservation
		err error
	}

	cases := map[string]struct {
		args    args
		want    want
		service service
		kube    k8s.Client
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
		},
		"ExternalNameNotSet": {
			args: args{
				mg: serviceCredentialBinding("key", withServiceInstanceID(serviceInstanceGUID)),
			},
			want: want{
				mg: serviceCredentialBinding("key", withServiceInstanceID(serviceInstanceGUID)),
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
				return m
			},
		},
		"Boom!": {
			args: args{
				mg: serviceCredentialBinding("key", withExternalName(guid), withServiceInstanceID(serviceInstanceGUID)),
			},
			want: want{
				mg:  serviceCredentialBinding("key", withExternalName(guid)),
				obs: managed.ExternalObservation{},
				err: errors.Wrap(errBoom, errGet),
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				m.On("Get", guid).Return(
					fake.ServiceCredentialBindingNil,
					errBoom,
				)
				m.On("Single").Return(
					fake.ServiceCredentialBindingNil,
					errBoom,
				)
				return m
			},
		},
		"NotFound": {
			args: args{
				mg: serviceCredentialBinding("key", withExternalName(guid), withServiceInstanceID(serviceInstanceGUID)),
			},
			want: want{
				mg:  serviceCredentialBinding("key", withExternalName(guid)),
				obs: managed.ExternalObservation{ResourceExists: false},
				err: nil,
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				m.On("Get", guid).Return(
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
		},
		"Successful": {
			args: args{
				mg: serviceCredentialBinding("key", withExternalName(guid), withServiceInstanceID(serviceInstanceGUID)),
			},
			want: want{
				mg: serviceCredentialBinding(
					"key",
					withExternalName(guid),
					withStatus(guid),
					withServiceInstanceID(serviceInstanceGUID),
					withConditions(xpv1.Available()),
				),
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true, ConnectionDetails: managed.ConnectionDetails{}},
				err: nil,
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				m.On("Get", guid).Return(
					&fake.NewServiceCredentialBinding("key").SetName(name).SetGUID(guid).SetServiceInstanceRef(serviceInstanceGUID).SetLastOperation(v1alpha1.LastOperationCreate, v1alpha1.LastOperationSucceeded).ServiceCredentialBinding,
					nil,
				)
				m.On("Single").Return(
					&fake.NewServiceCredentialBinding("key").SetName(name).SetGUID(guid).SetServiceInstanceRef(serviceInstanceGUID).SetLastOperation(v1alpha1.LastOperationCreate, v1alpha1.LastOperationSucceeded).ServiceCredentialBinding,
					nil,
				)
				m.On("GetDetails", guid).Return(
					fake.NewServiceCredentialBindingDetails(guid),
					nil,
				)
				return m
			},
		},
		"CreateFailed": {
			args: args{
				mg: serviceCredentialBinding("key", withExternalName(guid), withServiceInstanceID(serviceInstanceGUID)),
			},
			want: want{
				mg: serviceCredentialBinding(
					"key",
					withExternalName(guid),
					withServiceInstanceID(serviceInstanceGUID),
					withStatus(guid),
					withConditions(xpv1.Available()),
				),
				obs: managed.ExternalObservation{ResourceExists: false, ResourceUpToDate: true},
				err: nil,
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				m.On("Get", guid).Return(
					&fake.NewServiceCredentialBinding("key").SetName(name).SetGUID(guid).SetServiceInstanceRef(serviceInstanceGUID).SetLastOperation(v1alpha1.LastOperationCreate, v1alpha1.LastOperationFailed).ServiceCredentialBinding,
					nil,
				)
				m.On("Single").Return(
					&fake.NewServiceCredentialBinding("key").SetName(name).SetGUID(guid).SetServiceInstanceRef(serviceInstanceGUID).SetLastOperation(v1alpha1.LastOperationCreate, v1alpha1.LastOperationFailed).ServiceCredentialBinding,
					nil,
				)
				return m
			},
		},
		"UpdateFailed": {
			args: args{
				mg: serviceCredentialBinding("key", withExternalName(guid), withServiceInstanceID(serviceInstanceGUID)),
			},
			want: want{
				mg: serviceCredentialBinding("key",
					withExternalName(guid),
					withServiceInstanceID(serviceInstanceGUID),
					withStatus(guid),
					withConditions(xpv1.Available()),
				),
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: false},
				err: nil,
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				m.On("Get", guid).Return(
					&fake.NewServiceCredentialBinding("key").SetName(name).SetGUID(guid).SetServiceInstanceRef(serviceInstanceGUID).SetLastOperation(v1alpha1.LastOperationUpdate, v1alpha1.LastOperationFailed).ServiceCredentialBinding,
					nil,
				)
				m.On("Single").Return(
					&fake.NewServiceCredentialBinding("key").SetName(name).SetGUID(guid).SetServiceInstanceRef(serviceInstanceGUID).SetLastOperation(v1alpha1.LastOperationUpdate, v1alpha1.LastOperationFailed).ServiceCredentialBinding,
					nil,
				)
				return m
			},
		},
		"InProgress": {
			args: args{
				mg: serviceCredentialBinding("key", withExternalName(guid), withServiceInstanceID(serviceInstanceGUID)),
			},
			want: want{
				mg: serviceCredentialBinding("key",
					withExternalName(guid),
					withStatus(guid),
					withServiceInstanceID(serviceInstanceGUID),
					withConditions(xpv1.Unavailable()),
				),
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
				err: nil,
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				m.On("Get", guid).Return(
					&fake.NewServiceCredentialBinding("key").SetName(name).SetGUID(guid).SetServiceInstanceRef(serviceInstanceGUID).SetLastOperation(v1alpha1.LastOperationCreate, v1alpha1.LastOperationInProgress).ServiceCredentialBinding,
					nil,
				)
				m.On("Single").Return(
					&fake.NewServiceCredentialBinding("key").SetName(name).SetGUID(guid).SetServiceInstanceRef(serviceInstanceGUID).SetLastOperation(v1alpha1.LastOperationCreate, v1alpha1.LastOperationInProgress).ServiceCredentialBinding,
					nil,
				)
				return m
			},
		}}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			t.Logf("Testing: %s", t.Name())
			c := &external{
				kube: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(nil),
				},
				scbClient: tc.service(),
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

func TestCreate(t *testing.T) {
	type service func() *fake.MockServiceCredentialBinding
	type job func() *fake.MockJob
	type args struct {
		mg resource.Managed
	}

	type want struct {
		mg  resource.Managed
		obs managed.ExternalCreation
		err error
	}

	cases := map[string]struct {
		args    args
		want    want
		service service
		job
		kube k8s.Client
	}{
		"Successful": {
			args: args{
				mg: serviceCredentialBinding("key", withServiceInstanceID(serviceInstanceGUID)),
			},
			want: want{
				mg:  serviceCredentialBinding("key", withConditions(xpv1.Creating()), withExternalName(guid), withServiceInstanceID(serviceInstanceGUID)),
				obs: managed.ExternalCreation{},
				err: nil,
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				m.On("Create").Return(
					"JOB123",
					&fake.NewServiceCredentialBinding("key").SetName(name).SetGUID(guid).SetServiceInstanceRef(serviceInstanceGUID).ServiceCredentialBinding,
					nil,
				)
				m.On("Single").Return(
					&fake.NewServiceCredentialBinding("key").SetName(name).SetGUID(guid).SetServiceInstanceRef(serviceInstanceGUID).ServiceCredentialBinding,
					nil,
				)
				m.On("PollComplete").Return(nil)
				return m
			},
		},
		"Should fail if Service Instance is missing": {
			args: args{
				mg: serviceCredentialBinding("key"),
			},
			want: want{
				mg: serviceCredentialBinding("key",
					withConditions(xpv1.Creating()),
				),
				obs: managed.ExternalCreation{},
				err: errors.Wrap(errServiceInstanceMissing, errCreate),
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}

				m.On("Create").Return(
					"JOB123",
					&fake.NewServiceCredentialBinding("key").SetName(name).SetGUID(guid).SetServiceInstanceRef(serviceInstanceGUID).ServiceCredentialBinding,
					nil,
				)

				m.On("Single").Return(
					&fake.NewServiceCredentialBinding("key").SetName(name).SetGUID(guid).SetServiceInstanceRef(serviceInstanceGUID).ServiceCredentialBinding,
					nil,
				)
				m.On("PollComplete").Return(nil)

				return m
			},
		},
		"Should fail if App is missing for type app": {
			args: args{
				mg: serviceCredentialBinding("app", withServiceInstanceID(serviceInstanceGUID)),
			},
			want: want{
				mg: serviceCredentialBinding("app", withServiceInstanceID(serviceInstanceGUID),
					withConditions(xpv1.Creating()),
				),
				obs: managed.ExternalCreation{},
				err: errors.Wrap(errAppMissing, errCreate),
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}

				m.On("Create").Return(
					"JOB123",
					&fake.NewServiceCredentialBinding("app").SetName(name).SetGUID(guid).SetServiceInstanceRef(serviceInstanceGUID).ServiceCredentialBinding,
					nil,
				)

				m.On("Single").Return(
					&fake.NewServiceCredentialBinding("app").SetName(name).SetGUID(guid).SetServiceInstanceRef(serviceInstanceGUID).ServiceCredentialBinding,
					nil,
				)
				m.On("PollComplete").Return(nil)

				return m
			},
		},
		"CannotPollCreationJob": {
			args: args{
				mg: serviceCredentialBinding("key", withServiceInstanceID(serviceInstanceGUID)),
			},
			want: want{
				mg:  serviceCredentialBinding("key", withServiceInstanceID(serviceInstanceGUID), withConditions(xpv1.Creating())),
				obs: managed.ExternalCreation{},
				err: errors.Wrap(errBoom, errCreate),
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}

				m.On("Create").Return(
					"JOB123",
					&fake.NewServiceCredentialBinding("key").SetName(name).SetGUID(guid).SetServiceInstanceRef(serviceInstanceGUID).ServiceCredentialBinding,
					nil,
				)

				m.On("Single").Return(
					&fake.NewServiceCredentialBinding("key").SetName(name).SetGUID(guid).SetServiceInstanceRef(serviceInstanceGUID).ServiceCredentialBinding,
					nil,
				)
				m.On("PollComplete").Return(errBoom)

				return m
			},
		},
		"AlreadyExist": {
			args: args{
				mg: serviceCredentialBinding("key", withServiceInstanceID(serviceInstanceGUID)),
			},
			want: want{
				mg:  serviceCredentialBinding("key", withServiceInstanceID(serviceInstanceGUID), withConditions(xpv1.Creating())),
				obs: managed.ExternalCreation{},
				err: errors.Wrap(errBoom, errCreate),
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				m.On("Create").Return(
					"JOB123",
					errBoom,
				)
				m.On("Single").Return(
					&fake.NewServiceCredentialBinding("key").SetName(name).SetGUID(guid).SetServiceInstanceRef(serviceInstanceGUID).ServiceCredentialBinding,
					nil,
				)
				m.On("Get").Return(
					&fake.NewServiceCredentialBinding("key").SetName(name).SetGUID(guid).SetServiceInstanceRef(serviceInstanceGUID).ServiceCredentialBinding,
					nil,
				)
				m.On("PollComplete").Return(nil)

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
	type job func() *fake.MockJob
	type args struct {
		mg resource.Managed
	}

	type want struct {
		mg  resource.Managed
		obs managed.ExternalUpdate
		err error
	}

	cases := map[string]struct {
		args    args
		want    want
		service service
		job
		kube k8s.Client
	}{

		"DoesNotExist": {
			args: args{
				mg: serviceCredentialBinding("key", withServiceInstanceID(serviceInstanceGUID), withExternalName(guid), withStatus(guid)),
			},
			want: want{
				mg:  serviceCredentialBinding("key", withServiceInstanceID(serviceInstanceGUID), withExternalName(guid), withStatus(guid), withConditions(xpv1.Deleting())),
				obs: managed.ExternalUpdate{},
				err: errors.Wrap(errBoom, errDelete),
			},
			service: func() *fake.MockServiceCredentialBinding {
				m := &fake.MockServiceCredentialBinding{}
				m.On("Delete", guid).Return(
					"",
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
				scbClient: tc.service(),
			}
			err := c.Delete(context.Background(), tc.args.mg)

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
			if diff := cmp.Diff(tc.want.mg, tc.args.mg); diff != "" {
				t.Errorf("Observe(...): -want, +got:\n%s", diff)
			}
		})
	}
}
