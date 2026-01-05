package serviceroutebinding

import (
	"context"
	"errors"
	"fmt"
	"testing"

	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/mock"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/fake"
)

var (
	errBoom             = errors.New("boom")
	name                = "my-service-route-binding"
	guid                = "2d8b0d04-d537-4e4e-8c6f-f09ca0e7f56f"
	routeGUID           = "3d8b0d04-d537-4e4e-8c6f-f09ca0e7f56f"
	serviceInstanceGUID = "4d8b0d04-d537-4e4e-8c6f-f09ca0e7f56f"
	routeServiceURL     = "https://route-service.example.com"
)

type modifier func(*v1alpha1.ServiceRouteBinding)

func withExternalName(name string) modifier {
	return func(r *v1alpha1.ServiceRouteBinding) {
		r.ObjectMeta.Annotations[meta.AnnotationKeyExternalName] = name
	}
}

func withRouteID(guid string) modifier {
	return func(r *v1alpha1.ServiceRouteBinding) {
		r.Spec.ForProvider.Route = guid
	}
}

func withServiceInstanceID(guid string) modifier {
	return func(r *v1alpha1.ServiceRouteBinding) {
		r.Spec.ForProvider.ServiceInstance = guid
	}
}

func withConditions(c ...xpv1.Condition) modifier {
	return func(i *v1alpha1.ServiceRouteBinding) { i.Status.SetConditions(c...) }
}

func withStatus(guid string) modifier {
	o := v1alpha1.ServiceRouteBindingObservation{}
	o.GUID = guid
	o.Route = routeGUID
	o.ServiceInstance = serviceInstanceGUID

	return func(r *v1alpha1.ServiceRouteBinding) {
		r.Status.AtProvider = o
	}
}

func serviceRouteBinding(m ...modifier) *v1alpha1.ServiceRouteBinding {
	r := &v1alpha1.ServiceRouteBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Finalizers:  []string{},
			Annotations: map[string]string{},
		},
		Spec: v1alpha1.ServiceRouteBindingSpec{
			ForProvider: v1alpha1.ServiceRouteBindingParameters{},
		},
		Status: v1alpha1.ServiceRouteBindingStatus{
			AtProvider: v1alpha1.ServiceRouteBindingObservation{},
		},
	}

	for _, rm := range m {
		rm(r)
	}
	return r
}

func TestObserve(t *testing.T) {
	type service func() *fake.MockServiceRouteBinding
	type args struct {
		mg resource.Managed
	}

	type want struct {
		mg  resource.Managed
		obs managed.ExternalObservation
		err error
	}

	srb := serviceRouteBinding(withExternalName(guid), withRouteID(routeGUID), withServiceInstanceID(serviceInstanceGUID))
	srbAvailable := serviceRouteBinding(
		withExternalName(guid),
		withStatus(guid),
		withRouteID(routeGUID),
		withServiceInstanceID(serviceInstanceGUID),
		withConditions(xpv1.Available()),
	)

	cfSucceeded := func() *cfresource.ServiceRouteBinding {
		return &fake.NewServiceRouteBinding().
			SetGUID(guid).
			SetRouteRef(routeGUID).
			SetServiceInstanceRef(serviceInstanceGUID).
			SetRouteServiceURL(routeServiceURL).
			SetLastOperation(v1alpha1.LastOperationCreate, v1alpha1.LastOperationSucceeded).
			ServiceRouteBinding
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
			service: func() *fake.MockServiceRouteBinding {
				m := &fake.MockServiceRouteBinding{}
				return m
			},
		},
		"ExternalNameNotSet": {
			args: args{
				mg: serviceRouteBinding(),
			},
			want: want{
				mg: serviceRouteBinding(),
				obs: managed.ExternalObservation{
					ResourceExists: false,
				},
				err: nil,
			},
			service: func() *fake.MockServiceRouteBinding {
				m := &fake.MockServiceRouteBinding{}
				m.On("Single", mock.Anything, mock.Anything).Return(
					nil,
					fake.ErrNoResultReturned,
				)
				m.On("Get", mock.Anything, "").Return(
					nil,
					fake.ErrNoResultReturned,
				)
				return m
			},
		},
		"Boom!": {
			args: args{
				mg: srb.DeepCopy(),
			},
			want: want{
				mg:  serviceRouteBinding(withExternalName(guid)),
				obs: managed.ExternalObservation{},
				err: fmt.Errorf(errGet, errBoom),
			},
			service: func() *fake.MockServiceRouteBinding {
				m := &fake.MockServiceRouteBinding{}
				m.On("Get", mock.Anything, guid).Return(
					nil,
					errBoom,
				)
				m.On("Single", mock.Anything, mock.Anything).Return(
					nil,
					errBoom,
				)
				return m
			},
		},
		"NotFound": {
			args: args{
				mg: srb.DeepCopy(),
			},
			want: want{
				mg:  serviceRouteBinding(withExternalName(guid)),
				obs: managed.ExternalObservation{ResourceExists: false},
				err: nil,
			},
			service: func() *fake.MockServiceRouteBinding {
				m := &fake.MockServiceRouteBinding{}
				m.On("Get", mock.Anything, guid).Return(
					nil,
					fake.ErrNoResultReturned,
				)
				m.On("Single", mock.Anything, mock.Anything).Return(
					nil,
					fake.ErrNoResultReturned,
				)
				return m
			},
			kube: &test.MockClient{},
		},
		"Successful": {
			args: args{
				mg: srb.DeepCopy(),
			},
			want: want{
				mg:  srbAvailable.DeepCopy(),
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
				err: nil,
			},
			service: func() *fake.MockServiceRouteBinding {
				m := &fake.MockServiceRouteBinding{}
				m.On("Get", mock.Anything, guid).Return(
					cfSucceeded(),
					nil,
				)
				m.On("Single", mock.Anything, mock.Anything).Return(
					cfSucceeded(),
					nil,
				)
				return m
			},
		},
		"InProgress": {
			args: args{
				mg: srb.DeepCopy(),
			},
			want: want{
				mg: serviceRouteBinding(
					withExternalName(guid),
					withStatus(guid),
					withConditions(xpv1.Unavailable()),
				),
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
				err: nil,
			},
			service: func() *fake.MockServiceRouteBinding {
				m := &fake.MockServiceRouteBinding{}
				inProgress := &fake.NewServiceRouteBinding().
					SetGUID(guid).
					SetRouteRef(routeGUID).
					SetServiceInstanceRef(serviceInstanceGUID).
					SetLastOperation(v1alpha1.LastOperationCreate, v1alpha1.LastOperationInProgress).
					ServiceRouteBinding
				m.On("Get", mock.Anything, guid).Return(
					inProgress,
					nil,
				)
				return m
			},
		},
		"CreateFailed": {
			args: args{
				mg: srb.DeepCopy(),
			},
			want: want{
				mg: serviceRouteBinding(
					withExternalName(guid),
					withStatus(guid),
					withConditions(xpv1.Unavailable()),
				),
				obs: managed.ExternalObservation{ResourceExists: false, ResourceUpToDate: false},
				err: nil,
			},
			service: func() *fake.MockServiceRouteBinding {
				m := &fake.MockServiceRouteBinding{}
				failed := &fake.NewServiceRouteBinding().
					SetGUID(guid).
					SetRouteRef(routeGUID).
					SetServiceInstanceRef(serviceInstanceGUID).
					SetLastOperation(v1alpha1.LastOperationCreate, v1alpha1.LastOperationFailed).
					ServiceRouteBinding
				m.On("Get", mock.Anything, guid).Return(
					failed,
					nil,
				)
				return m
			},
		},
		"DeleteSucceeded": {
			args: args{
				mg: srb.DeepCopy(),
			},
			want: want{
				mg: serviceRouteBinding(
					withExternalName(guid),
					withStatus(guid),
					withConditions(xpv1.Deleting()),
				),
				obs: managed.ExternalObservation{ResourceExists: false, ResourceUpToDate: true},
				err: nil,
			},
			service: func() *fake.MockServiceRouteBinding {
				m := &fake.MockServiceRouteBinding{}
				deleted := &fake.NewServiceRouteBinding().
					SetGUID(guid).
					SetRouteRef(routeGUID).
					SetServiceInstanceRef(serviceInstanceGUID).
					SetLastOperation(v1alpha1.LastOperationDelete, v1alpha1.LastOperationSucceeded).
					ServiceRouteBinding
				m.On("Get", mock.Anything, guid).Return(
					deleted,
					nil,
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
					MockUpdate: test.NewMockUpdateFn(nil),
				},
				srbClient: tc.service(),
			}
			obs, err := c.Observe(context.Background(), tc.args.mg)

			if tc.want.err != nil && err != nil {
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
	type service func() *fake.MockServiceRouteBinding
	type args struct {
		mg resource.Managed
	}

	type want struct {
		mg  resource.Managed
		obs managed.ExternalCreation
		err error
	}

	srb := serviceRouteBinding(withRouteID(routeGUID), withServiceInstanceID(serviceInstanceGUID))

	cases := map[string]struct {
		args    args
		want    want
		service service
	}{
		"Successful": {
			args: args{
				mg: srb.DeepCopy(),
			},
			want: want{
				mg:  serviceRouteBinding(withRouteID(routeGUID), withServiceInstanceID(serviceInstanceGUID), withExternalName(guid)),
				obs: managed.ExternalCreation{},
				err: nil,
			},
			service: func() *fake.MockServiceRouteBinding {
				m := &fake.MockServiceRouteBinding{}
				created := &fake.NewServiceRouteBinding().
					SetGUID(guid).
					SetRouteRef(routeGUID).
					SetServiceInstanceRef(serviceInstanceGUID).
					SetLastOperation(v1alpha1.LastOperationCreate, v1alpha1.LastOperationInProgress).
					ServiceRouteBinding
				m.On("Create", mock.Anything, mock.Anything).Return(
					"", // no job GUID
					created,
					nil,
				)
				m.On("Single", mock.Anything, mock.Anything).Return(
					created,
					nil,
				)
				return m
			},
		},
		"CreateFailed": {
			args: args{
				mg: srb.DeepCopy(),
			},
			want: want{
				mg:  serviceRouteBinding(withRouteID(routeGUID), withServiceInstanceID(serviceInstanceGUID)),
				obs: managed.ExternalCreation{},
				err: fmt.Errorf(errCreate, errBoom),
			},
			service: func() *fake.MockServiceRouteBinding {
				m := &fake.MockServiceRouteBinding{}
				m.On("Create", mock.Anything, mock.Anything).Return(
					"",
					nil,
					errBoom,
				)
				return m
			},
		},
		"MissingRouteGUID": {
			args: args{
				mg: serviceRouteBinding(withServiceInstanceID(serviceInstanceGUID)), // route missing
			},
			want: want{
				mg:  serviceRouteBinding(withServiceInstanceID(serviceInstanceGUID)),
				obs: managed.ExternalCreation{},
				err: fmt.Errorf(errCreate, fmt.Errorf(errMissingRelationshipGUIDs, "", serviceInstanceGUID)),
			},
			service: func() *fake.MockServiceRouteBinding {
				m := &fake.MockServiceRouteBinding{}
				return m
			},
		},
		"MissingServiceInstanceGUID": {
			args: args{
				mg: serviceRouteBinding(withRouteID(routeGUID)), // service instance missing
			},
			want: want{
				mg:  serviceRouteBinding(withRouteID(routeGUID)),
				obs: managed.ExternalCreation{},
				err: fmt.Errorf(errCreate, fmt.Errorf(errMissingRelationshipGUIDs, routeGUID, "")),
			},
			service: func() *fake.MockServiceRouteBinding {
				m := &fake.MockServiceRouteBinding{}
				return m
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			t.Logf("Testing: %s", t.Name())
			c := &external{
				kube: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(nil),
				},
				srbClient: tc.service(),
			}
			obs, err := c.Create(context.Background(), tc.args.mg)

			if tc.want.err != nil && err != nil {
				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("Create(...): want error string != got error string:\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(tc.want.err, err); diff != "" {
					t.Errorf("Create(...): want error != got error:\n%s", diff)
				}
			}
			if diff := cmp.Diff(tc.want.obs, obs); diff != "" {
				t.Errorf("Create(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestUpdate(t *testing.T) {
	type service func() *fake.MockServiceRouteBinding
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
	}{
		"Successful": {
			args: args{
				mg: serviceRouteBinding(withServiceInstanceID(serviceInstanceGUID), withExternalName(guid)),
			},
			want: want{
				mg:  serviceRouteBinding(withServiceInstanceID(serviceInstanceGUID), withExternalName(guid)),
				obs: managed.ExternalUpdate{},
				err: nil,
			},
			service: func() *fake.MockServiceRouteBinding {
				m := &fake.MockServiceRouteBinding{}
				return m
			},
		},
		"EmptyExternalName": {
			args: args{
				mg: serviceRouteBinding(withServiceInstanceID(serviceInstanceGUID)),
			},
			want: want{
				mg:  serviceRouteBinding(withServiceInstanceID(serviceInstanceGUID)),
				obs: managed.ExternalUpdate{},
				err: nil,
			},
			service: func() *fake.MockServiceRouteBinding {
				m := &fake.MockServiceRouteBinding{}
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
				srbClient: tc.service(),
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

func TestDelete(t *testing.T) {
	type service func() *fake.MockServiceRouteBinding
	type args struct {
		mg resource.Managed
	}

	type want struct {
		mg  resource.Managed
		err error
	}

	cases := map[string]struct {
		args    args
		want    want
		service service
	}{
		"Successful": {
			args: args{
				mg: serviceRouteBinding(withExternalName(guid), withStatus(guid)),
			},
			want: want{
				mg:  serviceRouteBinding(withExternalName(guid), withStatus(guid), withConditions(xpv1.Deleting())),
				err: nil,
			},
			service: func() *fake.MockServiceRouteBinding {
				m := &fake.MockServiceRouteBinding{}
				m.On("Delete", mock.Anything, guid).Return(
					"", // no job GUID
					nil,
				)
				return m
			},
		},
		"DeleteFailed": {
			args: args{
				mg: serviceRouteBinding(withExternalName(guid), withStatus(guid)),
			},
			want: want{
				mg:  serviceRouteBinding(withExternalName(guid), withStatus(guid), withConditions(xpv1.Deleting())),
				err: fmt.Errorf(errDelete, errBoom),
			},
			service: func() *fake.MockServiceRouteBinding {
				m := &fake.MockServiceRouteBinding{}
				m.On("Delete", mock.Anything, guid).Return(
					"",
					errBoom,
				)
				return m
			},
		},
		"NotFound": {
			args: args{
				mg: serviceRouteBinding(withExternalName(guid), withStatus(guid)),
			},
			want: want{
				mg:  serviceRouteBinding(withExternalName(guid), withStatus(guid), withConditions(xpv1.Deleting())),
				err: nil,
			},
			service: func() *fake.MockServiceRouteBinding {
				m := &fake.MockServiceRouteBinding{}
				m.On("Delete", mock.Anything, guid).Return(
					"",
					fake.ErrNoResultReturned,
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
					MockUpdate: test.NewMockUpdateFn(nil),
				},
				srbClient: tc.service(),
			}
			_, err := c.Delete(context.Background(), tc.args.mg)

			if tc.want.err != nil && err != nil {
				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("Delete(...): want error string != got error string:\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(tc.want.err, err); diff != "" {
					t.Errorf("Delete(...): want error != got error:\n%s", diff)
				}
			}
		})
	}
}

func TestHandleObservationState(t *testing.T) {
	type args struct {
		binding *cfresource.ServiceRouteBinding
		cr      *v1alpha1.ServiceRouteBinding
	}

	type want struct {
		obs managed.ExternalObservation
		err error
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"Initial": {
			args: args{
				binding: &fake.NewServiceRouteBinding().
					SetLastOperation(v1alpha1.LastOperationCreate, v1alpha1.LastOperationInitial).
					ServiceRouteBinding,
				cr: serviceRouteBinding(),
			},
			want: want{
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
				err: nil,
			},
		},
		"InProgress": {
			args: args{
				binding: &fake.NewServiceRouteBinding().
					SetLastOperation(v1alpha1.LastOperationCreate, v1alpha1.LastOperationInProgress).
					ServiceRouteBinding,
				cr: serviceRouteBinding(),
			},
			want: want{
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
				err: nil,
			},
		},
		"CreateFailed": {
			args: args{
				binding: &fake.NewServiceRouteBinding().
					SetLastOperation(v1alpha1.LastOperationCreate, v1alpha1.LastOperationFailed).
					ServiceRouteBinding,
				cr: serviceRouteBinding(),
			},
			want: want{
				obs: managed.ExternalObservation{ResourceExists: false, ResourceUpToDate: false},
				err: nil,
			},
		},
		"UpdateFailed": {
			args: args{
				binding: &fake.NewServiceRouteBinding().
					SetLastOperation(v1alpha1.LastOperationUpdate, v1alpha1.LastOperationFailed).
					ServiceRouteBinding,
				cr: serviceRouteBinding(),
			},
			want: want{
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: false},
				err: nil,
			},
		},
		"DeleteFailed": {
			args: args{
				binding: &fake.NewServiceRouteBinding().
					SetLastOperation(v1alpha1.LastOperationDelete, v1alpha1.LastOperationFailed).
					ServiceRouteBinding,
				cr: serviceRouteBinding(),
			},
			want: want{
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
				err: nil,
			},
		},
		"DeleteSucceeded": {
			args: args{
				binding: &fake.NewServiceRouteBinding().
					SetLastOperation(v1alpha1.LastOperationDelete, v1alpha1.LastOperationSucceeded).
					ServiceRouteBinding,
				cr: serviceRouteBinding(),
			},
			want: want{
				obs: managed.ExternalObservation{ResourceExists: false, ResourceUpToDate: true},
				err: nil,
			},
		},
		"Succeeded": {
			args: args{
				binding: &fake.NewServiceRouteBinding().
					SetLastOperation(v1alpha1.LastOperationCreate, v1alpha1.LastOperationSucceeded).
					ServiceRouteBinding,
				cr: serviceRouteBinding(),
			},
			want: want{
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
				err: nil,
			},
		},
		"UnknownState": {
			args: args{
				binding: &fake.NewServiceRouteBinding().ServiceRouteBinding,
				cr:      serviceRouteBinding(),
			},
			want: want{
				obs: managed.ExternalObservation{},
				err: errors.New(errUnknownState),
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			obs, err := handleObservationState(tc.args.binding, tc.args.cr)

			if tc.want.err != nil && err != nil {
				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("handleObservationState(...): want error string != got error string:\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(tc.want.err, err); diff != "" {
					t.Errorf("handleObservationState(...): want error != got error:\n%s", diff)
				}
			}
			if diff := cmp.Diff(tc.want.obs, obs); diff != "" {
				t.Errorf("handleObservationState(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestIsNotFoundError(t *testing.T) {
	cases := map[string]struct {
		err  error
		want bool
	}{
		"Nil": {
			err:  nil,
			want: false,
		},
		"ErrNoResultsReturned": {
			err:  fake.ErrNoResultReturned,
			want: true,
		},
		"ErrExactlyOneResultNotReturned": {
			err:  fake.ErrExactlyOneResultNotReturned,
			want: true,
		},
		"CF-ResourceNotFound": {
			err:  errors.New("CF-ResourceNotFound: The resource could not be found"),
			want: true,
		},
		"ServiceRouteBindingNotFound": {
			err:  errors.New("service route binding not found"),
			want: true,
		},
		"OtherError": {
			err:  errBoom,
			want: false,
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			got := isNotFoundError(tc.err)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("isNotFoundError(...): -want, +got:\n%s", diff)
			}
		})
	}
}
