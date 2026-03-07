package serviceinstance

import (
	"context"
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/fake"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/serviceinstance"
)

var (
	errBoom         = errors.New("boom")
	name            = "my-service-instance"
	spaceGUID       = "a46808d1-d09a-4eef-add1-30872dec82f7"
	guid            = "2d8b0d04-d537-4e4e-8c6f-f09ca0e7f56f"
	servicePlan     = "c595293f-2696-438d-887e-053200ec47c8"
	jsonCredentials = `{"json":"bar"}`
)

type modifier func(*v1alpha1.ServiceInstance)

func withExternalName(name string) modifier {
	return func(r *v1alpha1.ServiceInstance) {
		r.ObjectMeta.Annotations[meta.AnnotationKeyExternalName] = name
	}
}

func withCredentials(credentials *string) modifier {
	return func(r *v1alpha1.ServiceInstance) {
		switch r.Spec.ForProvider.Type {
		case v1alpha1.ManagedService:
			r.Spec.ForProvider.JSONParams = credentials
		case v1alpha1.UserProvidedService:
			r.Spec.ForProvider.JSONCredentials = credentials
		}
	}
}

func withServicePlan(servicePlan v1alpha1.ServicePlanParameters) modifier {
	return func(r *v1alpha1.ServiceInstance) {
		r.Spec.ForProvider.ServicePlan = &servicePlan
	}
}

func withSpace(spaceGUID string) modifier {
	return func(r *v1alpha1.ServiceInstance) {
		r.Spec.ForProvider.Space = &spaceGUID
	}
}

func withConditions(c ...xpv1.Condition) modifier {
	return func(i *v1alpha1.ServiceInstance) { i.Status.SetConditions(c...) }
}

func withStatus(s v1alpha1.ServiceInstanceObservation) modifier {
	return func(r *v1alpha1.ServiceInstance) {
		r.Status.AtProvider = s
	}
}

func withParameters(params string) modifier {
	return func(r *v1alpha1.ServiceInstance) {
		r.Spec.ForProvider.JSONParams = &params
	}
}

func withDriftDetection(d bool) modifier {
	return func(r *v1alpha1.ServiceInstance) {
		r.Spec.EnableParameterDriftDetection = d
	}
}

func withDeletionTimestamp() modifier {
	return func(r *v1alpha1.ServiceInstance) {
		ts := metav1.Now()
		r.ObjectMeta.DeletionTimestamp = &ts
	}
}

func serviceInstance(typ string, m ...modifier) *v1alpha1.ServiceInstance {
	r := &v1alpha1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Finalizers:  []string{},
			Annotations: map[string]string{},
		},
		Spec: v1alpha1.ServiceInstanceSpec{
			ForProvider: v1alpha1.ServiceInstanceParameters{Type: v1alpha1.ServiceInstanceType(typ), Name: &name},
		},
		Status: v1alpha1.ServiceInstanceStatus{
			AtProvider: v1alpha1.ServiceInstanceObservation{},
		},
	}

	for _, rm := range m {
		rm(r)
	}
	return r
}

func TestObserve(t *testing.T) {
	type service func() *fake.MockServiceInstance
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
			service: func() *fake.MockServiceInstance {
				m := &fake.MockServiceInstance{}
				return m
			},
		},
		"ExternalNameNotSet": {
			args: args{
				mg: serviceInstance("managed", withSpace(spaceGUID), withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan})),
			},
			want: want{
				mg: serviceInstance("managed", withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan}), withSpace(spaceGUID)),
				obs: managed.ExternalObservation{
					ResourceExists: false,
				},
				err: nil,
			},
			service: func() *fake.MockServiceInstance {
				m := &fake.MockServiceInstance{}
				m.On("Single").Return(
					fake.ServiceInstanceNil,
					fake.ErrNoResultReturned,
				)
				return m
			},
		},
		"Boom!": {
			args: args{
				mg: serviceInstance("managed", withExternalName(guid), withSpace(spaceGUID), withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan})),
			},
			want: want{
				mg:  serviceInstance("managed", withExternalName(guid)),
				obs: managed.ExternalObservation{},
				err: errors.Wrap(errBoom, errGet),
			},
			service: func() *fake.MockServiceInstance {
				m := &fake.MockServiceInstance{}
				m.On("Get", guid).Return(
					fake.ServiceInstanceNil,
					errBoom,
				)
				m.On("Single").Return(
					fake.ServiceInstanceNil,
					errBoom,
				)
				return m
			},
		},
		"NotFound - Get by GUID when valid GUID is recorded": {
			args: args{
				mg: serviceInstance("managed", withExternalName(guid), withSpace(spaceGUID), withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan})),
			},
			want: want{
				mg:  serviceInstance("managed", withExternalName(guid)),
				obs: managed.ExternalObservation{ResourceExists: false},
				err: nil,
			},
			service: func() *fake.MockServiceInstance {
				m := &fake.MockServiceInstance{}
				m.On("Get", guid).Return(
					fake.ServiceInstanceNil,
					fake.ErrNoResultReturned,
				)
				m.On("Single").Return(
					fake.ServiceInstanceNil,
					errBoom,
				)
				return m
			},
			kube: &test.MockClient{},
		},

		"NotFound - fallback on Single when NO valid GUID is recorded in CR": {
			args: args{
				mg: serviceInstance("managed", withExternalName("not-guid"), withSpace(spaceGUID), withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan})),
			},
			want: want{
				mg:  serviceInstance("managed", withExternalName(guid)),
				obs: managed.ExternalObservation{ResourceExists: false},
				err: nil,
			},
			service: func() *fake.MockServiceInstance {
				m := &fake.MockServiceInstance{}
				m.On("Get", "not-guid").Return(
					fake.ServiceInstanceNil,
					errBoom,
				)
				m.On("Single").Return(
					fake.ServiceInstanceNil,
					fake.ErrNoResultReturned,
				)
				return m
			},
			kube: &test.MockClient{},
		},
		"Successful - Get by GUID": {
			args: args{
				mg: serviceInstance("managed", withExternalName(guid), withSpace(spaceGUID), withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan})),
			},
			want: want{
				mg: serviceInstance("managed",
					withExternalName(guid),
					withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan}),
					withStatus(v1alpha1.ServiceInstanceObservation{ID: &guid, ServicePlan: &servicePlan}),
					withConditions(xpv1.Available()),
				),
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
				err: nil,
			},
			service: func() *fake.MockServiceInstance {
				m := &fake.MockServiceInstance{}
				m.On("Get", guid).Return(
					&fake.NewServiceInstance("managed").SetName(name).SetGUID(guid).SetServicePlan(servicePlan).SetLastOperation(v1alpha1.LastOperationCreate, v1alpha1.LastOperationSucceeded).ServiceInstance,
					nil,
				)
				m.On("Single").Return(
					&fake.NewServiceInstance("managed").SetName(name).SetGUID(guid).SetServicePlan(servicePlan).SetLastOperation(v1alpha1.LastOperationCreate, v1alpha1.LastOperationSucceeded).ServiceInstance,
					nil,
				)
				m.On("GetManagedParameters", guid).Return(
					fake.JSONRawMessage(""),
					nil, // no error
				)
				return m
			},
		},
		"Successful - adopt by forProvider spec": {
			args: args{
				mg: serviceInstance("managed", withExternalName("not-guid"), withSpace(spaceGUID), withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan})),
			},
			want: want{
				mg: serviceInstance("managed",
					withExternalName(guid),
					withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan}),
					withStatus(v1alpha1.ServiceInstanceObservation{ID: &guid, ServicePlan: &servicePlan}),
					withConditions(xpv1.Available()),
				),
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
				err: nil,
			},
			service: func() *fake.MockServiceInstance {
				m := &fake.MockServiceInstance{}
				m.On("Get", "not-guid").Return(
					fake.ServiceInstanceNil,
					fake.ErrNoResultReturned,
				)
				m.On("Single").Return(
					&fake.NewServiceInstance("managed").SetName(name).SetGUID(guid).SetServicePlan(servicePlan).SetLastOperation(v1alpha1.LastOperationCreate, v1alpha1.LastOperationSucceeded).ServiceInstance,
					nil,
				)
				m.On("GetManagedParameters", guid).Return(
					fake.JSONRawMessage(""),
					nil, // no error
				)
				return m
			},
		},
		"CreateFailed": {
			args: args{
				mg: serviceInstance("managed", withExternalName(guid), withSpace(spaceGUID), withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan})),
			},
			want: want{
				mg: serviceInstance("managed",
					withExternalName(guid),
					withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan}),
					withStatus(v1alpha1.ServiceInstanceObservation{ID: &guid, ServicePlan: &servicePlan}),
					withConditions(xpv1.Available()),
				),
				obs: managed.ExternalObservation{ResourceExists: false, ResourceUpToDate: true},
				err: nil,
			},
			service: func() *fake.MockServiceInstance {
				m := &fake.MockServiceInstance{}
				m.On("Get", guid).Return(
					&fake.NewServiceInstance("managed").SetName(name).SetGUID(guid).SetServicePlan(servicePlan).SetLastOperation(v1alpha1.LastOperationCreate, v1alpha1.LastOperationFailed).ServiceInstance,
					nil,
				)
				m.On("Single").Return(
					&fake.NewServiceInstance("managed").SetName(name).SetGUID(guid).SetServicePlan(servicePlan).SetLastOperation(v1alpha1.LastOperationCreate, v1alpha1.LastOperationFailed).ServiceInstance,
					nil,
				)
				m.On("GetManagedParameters", guid).Return(
					fake.JSONRawMessage(""),
					nil, // no error
				)
				return m
			},
		},
		"UpdateFailed": {
			args: args{
				mg: serviceInstance("managed", withExternalName(guid), withSpace(spaceGUID), withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan})),
			},
			want: want{
				mg: serviceInstance("managed",
					withExternalName(guid),
					withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan}),
					withStatus(v1alpha1.ServiceInstanceObservation{ID: &guid, ServicePlan: &servicePlan}),
					withConditions(xpv1.Available()),
				),
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: false},
				err: nil,
			},
			service: func() *fake.MockServiceInstance {
				m := &fake.MockServiceInstance{}
				m.On("Get", guid).Return(
					&fake.NewServiceInstance("managed").SetName(name).SetGUID(guid).SetServicePlan(servicePlan).SetLastOperation(v1alpha1.LastOperationUpdate, v1alpha1.LastOperationFailed).ServiceInstance,
					nil,
				)
				m.On("Single").Return(
					&fake.NewServiceInstance("managed").SetName(name).SetGUID(guid).SetServicePlan(servicePlan).SetLastOperation(v1alpha1.LastOperationUpdate, v1alpha1.LastOperationFailed).ServiceInstance,
					nil,
				)
				m.On("GetManagedParameters", guid).Return(
					fake.JSONRawMessage(""),
					nil, // no error
				)
				return m
			},
		},
		"InProgress": {
			args: args{
				mg: serviceInstance("managed", withExternalName(guid), withSpace(spaceGUID), withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan})),
			},
			want: want{
				mg: serviceInstance("managed",
					withExternalName(guid),
					withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan}),
					withStatus(v1alpha1.ServiceInstanceObservation{ID: &guid, ServicePlan: &servicePlan}),
					withConditions(xpv1.Unavailable()),
				),
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
				err: nil,
			},
			service: func() *fake.MockServiceInstance {
				m := &fake.MockServiceInstance{}
				m.On("Get", guid).Return(
					&fake.NewServiceInstance("managed").SetName(name).SetGUID(guid).SetServicePlan(servicePlan).SetLastOperation(v1alpha1.LastOperationCreate, v1alpha1.LastOperationInProgress).ServiceInstance,
					nil,
				)
				m.On("Single").Return(
					&fake.NewServiceInstance("managed").SetName(name).SetGUID(guid).SetServicePlan(servicePlan).SetLastOperation(v1alpha1.LastOperationCreate, v1alpha1.LastOperationInProgress).ServiceInstance,
					nil,
				)
				m.On("GetManagedParameters", guid).Return(
					fake.JSONRawMessage(""),
					nil, // no error
				)
				return m
			},
		},
		"DeletionFastPath_Exists": {
			args: args{
				mg: serviceInstance("managed",
					withExternalName(guid),
					withSpace(spaceGUID),
					withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan}),
					withDeletionTimestamp(), // triggers meta.WasDeleted short-circuit
				),
			},
			want: want{
				mg: serviceInstance("managed",
					withExternalName(guid),
					withSpace(spaceGUID),
					withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan}),
					withDeletionTimestamp(),
					// Status updated by UpdateObservation before early return
					withStatus(v1alpha1.ServiceInstanceObservation{ID: &guid, ServicePlan: &servicePlan}),
				),
				// Early return only sets ResourceExists: true
				obs: managed.ExternalObservation{ResourceExists: true},
				err: nil,
			},
			service: func() *fake.MockServiceInstance {
				m := &fake.MockServiceInstance{}
				m.On("Get", guid).Return(
					// LastOperationFailed + Create would have produced ResourceExists:false in the normal path,
					// proving we exited early.
					&fake.NewServiceInstance("managed").
						SetName(name).
						SetGUID(guid).
						SetServicePlan(servicePlan).
						SetLastOperation(v1alpha1.LastOperationCreate, v1alpha1.LastOperationFailed).
						ServiceInstance,
					nil,
				)
				// Fallback shouldn't be called, keep safe default.
				m.On("Single").Return(fake.ServiceInstanceNil, fake.ErrNoResultReturned)
				return m
			},
		},
		"DriftDetectionLoop": {
			args: args{
				mg: serviceInstance("managed", withExternalName(guid), withSpace(spaceGUID), withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan}), withParameters("{\"foo\":\"bar\", \"baz\": 1}"), withDriftDetection(true)),
			},
			want: want{
				mg: serviceInstance("managed",
					withExternalName(guid),
					withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan}),
					withStatus(v1alpha1.ServiceInstanceObservation{ID: &guid, ServicePlan: &servicePlan, Credentials: iSha256(*fake.JSONRawMessage("{\"foo\":\"bar\"}"))}),
					withConditions(xpv1.Available()),
					withParameters("{\"foo\":\"bar\", \"baz\": 1}"),
					withDriftDetection(true),
				),
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: false},
				err: nil,
			},
			service: func() *fake.MockServiceInstance {
				m := &fake.MockServiceInstance{}
				m.On("Get", guid).Return(
					&fake.NewServiceInstance("managed").SetName(name).SetGUID(guid).SetServicePlan(servicePlan).SetLastOperation(v1alpha1.LastOperationCreate, v1alpha1.LastOperationSucceeded).ServiceInstance,
					nil,
				)
				m.On("Single").Return(
					&fake.NewServiceInstance("managed").SetName(name).SetGUID(guid).SetServicePlan(servicePlan).SetLastOperation(v1alpha1.LastOperationCreate, v1alpha1.LastOperationSucceeded).ServiceInstance,
					nil,
				)
				m.On("GetManagedParameters", guid).Return(
					fake.JSONRawMessage("{\"foo\":\"bar\"}"),
					nil, // no error
				)
				return m
			},
		},
		"DriftDetectionBreak": {
			args: args{
				mg: serviceInstance("managed", withExternalName(guid), withSpace(spaceGUID), withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan}), withParameters("{\"foo\":\"bar\", \"baz\": 1}"), withDriftDetection(false), withStatus(v1alpha1.ServiceInstanceObservation{Credentials: iSha256([]byte("{\"foo\":\"bar\", \"baz\": 1}"))})),
			},
			want: want{
				mg: serviceInstance("managed",
					withExternalName(guid),
					withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan}),
					withStatus(v1alpha1.ServiceInstanceObservation{ID: &guid, ServicePlan: &servicePlan, Credentials: iSha256([]byte("{\"foo\":\"bar\", \"baz\": 1}"))}),
					withConditions(xpv1.Available()),
					withParameters("{\"foo\":\"bar\", \"baz\": 1}"),
					withDriftDetection(false),
				),
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
				err: nil,
			},
			service: func() *fake.MockServiceInstance {
				m := &fake.MockServiceInstance{}
				m.On("Get", guid).Return(
					&fake.NewServiceInstance("managed").SetName(name).SetGUID(guid).SetServicePlan(servicePlan).SetLastOperation(v1alpha1.LastOperationCreate, v1alpha1.LastOperationSucceeded).ServiceInstance,
					nil,
				)
				m.On("Single").Return(
					&fake.NewServiceInstance("managed").SetName(name).SetGUID(guid).SetServicePlan(servicePlan).SetLastOperation(v1alpha1.LastOperationCreate, v1alpha1.LastOperationSucceeded).ServiceInstance,
					nil,
				)
				m.On("GetManagedParameters", guid).Return(
					fake.JSONRawMessage("{\"foo\":\"bar\"}"),
					nil, // no error
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
				serviceinstance: &serviceinstance.Client{
					ServiceInstance: tc.service(),
					Job:             nil,
				},
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

type timeoutError struct {
	timeout bool
}

func (e *timeoutError) Error() string { return "timeout error" }
func (e *timeoutError) Timeout() bool { return e.timeout }

func TestCreate(t *testing.T) {
	type service func() *fake.MockServiceInstance
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
				mg: serviceInstance("managed", withSpace(spaceGUID), withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan})),
			},
			want: want{
				mg:  serviceInstance("managed", withSpace(spaceGUID), withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan}), withConditions(xpv1.Creating()), withExternalName(guid)),
				obs: managed.ExternalCreation{},
				err: nil,
			},
			service: func() *fake.MockServiceInstance {
				m := &fake.MockServiceInstance{}
				m.On("CreateManaged").Return(
					"JOB123",
					nil,
				)
				m.On("Single").Return(
					&fake.NewServiceInstance("managed").SetName(name).SetGUID(guid).SetServicePlan(servicePlan).ServiceInstance,
					nil,
				)
				m.On("GetManagedParameters", guid).Return(
					nil,
					nil, // no error
				)
				return m
			},
			job: func() *fake.MockJob {
				m := &fake.MockJob{}
				m.On("PollComplete").Return(nil)
				return m
			},
		},
		"SuccessfulWithParams": {
			args: args{
				mg: serviceInstance("managed", withSpace(spaceGUID), withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan}), withCredentials(&jsonCredentials)),
			},
			want: want{
				mg:  serviceInstance("managed", withSpace(spaceGUID), withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan}), withCredentials(&jsonCredentials), withConditions(xpv1.Creating()), withExternalName(guid), withStatus(v1alpha1.ServiceInstanceObservation{Credentials: iSha256([]byte(jsonCredentials))})),
				obs: managed.ExternalCreation{},
				err: nil,
			},
			service: func() *fake.MockServiceInstance {
				m := &fake.MockServiceInstance{}
				m.On("CreateManaged").Return(
					"JOB123",
					nil,
				)
				m.On("Single").Return(
					&fake.NewServiceInstance("managed").SetName(name).SetGUID(guid).SetServicePlan(servicePlan).ServiceInstance,
					nil,
				)
				m.On("GetManagedParameters", guid).Return(
					fake.JSONRawMessage(jsonCredentials),
					nil, // no error
				)
				return m
			},
			job: func() *fake.MockJob {
				m := &fake.MockJob{}
				m.On("PollComplete").Return(nil)
				return m
			},
		},
		"HTTPClientTimeout": {
			args: args{
				mg: serviceInstance("managed", withSpace(spaceGUID), withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan}), withCredentials(&jsonCredentials)),
			},
			want: want{
				mg:  serviceInstance("managed", withSpace(spaceGUID), withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan}), withCredentials(&jsonCredentials), withConditions(xpv1.Creating()), withExternalName(guid), withStatus(v1alpha1.ServiceInstanceObservation{Credentials: iSha256([]byte(jsonCredentials))})),
				obs: managed.ExternalCreation{},
				err: nil,
			},
			service: func() *fake.MockServiceInstance {
				m := &fake.MockServiceInstance{}
				m.On("CreateManaged").Return(
					"JOB123",
					nil,
				)
				m.On("Single").Return(
					&fake.NewServiceInstance("managed").SetName(name).SetGUID(guid).SetServicePlan(servicePlan).ServiceInstance,
					nil,
				)
				m.On("GetManagedParameters", guid).Return(
					fake.JSONRawMessage(jsonCredentials),
					nil, // no error
				)
				return m
			},
			job: func() *fake.MockJob {
				m := &fake.MockJob{}
				m.On("PollComplete").Return(&url.Error{
					Err: &timeoutError{true},
				})
				return m
			},
		},
		"CannotPollCreationJob": {
			args: args{
				mg: serviceInstance("managed", withSpace(spaceGUID), withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan})),
			},
			want: want{
				mg:  serviceInstance("managed", withSpace(spaceGUID), withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan}), withConditions(xpv1.Creating())),
				obs: managed.ExternalCreation{},
				err: errors.Wrap(errBoom, errCreate),
			},
			service: func() *fake.MockServiceInstance {
				m := &fake.MockServiceInstance{}
				m.On("CreateManaged").Return(
					"JOB123",
					nil,
				)
				m.On("Single").Return(
					&fake.NewServiceInstance("managed").SetName(name).SetGUID(guid).SetServicePlan(servicePlan).ServiceInstance,
					nil,
				)
				m.On("GetManagedParameters", guid).Return(
					nil,
					nil, // no error
				)
				return m
			},
			job: func() *fake.MockJob {
				m := &fake.MockJob{}
				m.On("PollComplete").Return(errBoom)
				return m
			},
		},
		"AlreadyExist": {
			args: args{
				mg: serviceInstance("managed", withSpace(spaceGUID), withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan})),
			},
			want: want{
				mg:  serviceInstance("managed", withSpace(spaceGUID), withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan}), withConditions(xpv1.Creating())),
				obs: managed.ExternalCreation{},
				err: errors.Wrap(errBoom, errCreate),
			},
			service: func() *fake.MockServiceInstance {
				m := &fake.MockServiceInstance{}
				m.On("CreateManaged").Return(
					"JOB123",
					errBoom,
				)
				m.On("Single").Return(
					&fake.NewServiceInstance("managed").SetName(name).SetGUID(guid).SetServicePlan(servicePlan).ServiceInstance,
					nil,
				)
				m.On("GetManagedParameters", guid).Return(
					nil,
					nil, // no error
				)
				return m
			},
			job: func() *fake.MockJob {
				m := &fake.MockJob{}
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
				serviceinstance: &serviceinstance.Client{
					ServiceInstance: tc.service(),
					Job:             tc.job(),
				},
			}
			obs, err := c.Create(context.Background(), tc.args.mg)

			if tc.want.err != nil && err != nil {
				// the case where our mock server returns error.
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
			if diff := cmp.Diff(tc.want.mg, tc.args.mg); diff != "" {
				t.Errorf("Create(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestUpdate(t *testing.T) {
	type service func() *fake.MockServiceInstance
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
		"Successful": {
			args: args{
				mg: serviceInstance("managed", withSpace(spaceGUID), withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan}), withExternalName(guid), withStatus(v1alpha1.ServiceInstanceObservation{ID: &guid})),
			},
			want: want{
				mg:  serviceInstance("managed", withSpace(spaceGUID), withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan}), withExternalName(guid), withStatus(v1alpha1.ServiceInstanceObservation{ID: &guid})),
				obs: managed.ExternalUpdate{},
				err: nil,
			},
			service: func() *fake.MockServiceInstance {
				m := &fake.MockServiceInstance{}
				m.On("UpdateManaged", guid).Return(
					"JOB123",
					nil,
				)
				m.On("Get", guid).Return(
					&fake.NewServiceInstance("managed").SetName(name).SetGUID(guid).SetServicePlan(servicePlan).ServiceInstance,
					nil,
				)
				m.On("GetManagedParameters", guid).Return(
					nil,
					nil, // no error
				)
				return m
			},
			job: func() *fake.MockJob {
				m := &fake.MockJob{}
				m.On("PollComplete").Return(nil)
				return m
			},
		},
		"SuccessfulWithParams": {
			args: args{
				mg: serviceInstance("managed", withSpace(spaceGUID), withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan}), withExternalName(guid), withStatus(v1alpha1.ServiceInstanceObservation{ID: &guid}), withCredentials(&jsonCredentials)),
			},
			want: want{
				mg:  serviceInstance("managed", withSpace(spaceGUID), withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan}), withCredentials(&jsonCredentials), withExternalName(guid), withStatus(v1alpha1.ServiceInstanceObservation{ID: &guid, Credentials: iSha256([]byte(jsonCredentials))})),
				obs: managed.ExternalUpdate{},
				err: nil,
			},
			service: func() *fake.MockServiceInstance {
				m := &fake.MockServiceInstance{}
				m.On("UpdateManaged", guid).Return(
					"JOB123",
					nil,
				)
				m.On("Get", guid).Return(
					&fake.NewServiceInstance("managed").SetName(name).SetGUID(guid).SetServicePlan(servicePlan).ServiceInstance,
					nil,
				)
				m.On("GetManagedParameters", guid).Return(
					fake.JSONRawMessage(jsonCredentials),
					nil, // no error
				)
				return m
			},
			job: func() *fake.MockJob {
				m := &fake.MockJob{}
				m.On("PollComplete").Return(nil)
				return m
			},
		},
		"HTTPClientTimeout": {
			args: args{
				mg: serviceInstance("managed", withSpace(spaceGUID), withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan}), withExternalName(guid), withStatus(v1alpha1.ServiceInstanceObservation{ID: &guid}), withCredentials(&jsonCredentials)),
			},
			want: want{
				mg:  serviceInstance("managed", withSpace(spaceGUID), withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan}), withCredentials(&jsonCredentials), withExternalName(guid), withStatus(v1alpha1.ServiceInstanceObservation{ID: &guid, Credentials: iSha256([]byte(jsonCredentials))})),
				obs: managed.ExternalUpdate{},
				err: nil,
			},
			service: func() *fake.MockServiceInstance {
				m := &fake.MockServiceInstance{}
				m.On("UpdateManaged", guid).Return(
					"JOB123",
					nil,
				)
				m.On("Get", guid).Return(
					&fake.NewServiceInstance("managed").SetName(name).SetGUID(guid).SetServicePlan(servicePlan).ServiceInstance,
					nil,
				)
				m.On("GetManagedParameters", guid).Return(
					fake.JSONRawMessage(jsonCredentials),
					nil, // no error
				)
				return m
			},
			job: func() *fake.MockJob {
				m := &fake.MockJob{}
				m.On("PollComplete").Return(&url.Error{
					Err: &timeoutError{true},
				})
				return m
			},
		},
		"CannotPollCreationJob": {
			args: args{
				mg: serviceInstance("managed", withSpace(spaceGUID), withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan}), withExternalName(guid), withStatus(v1alpha1.ServiceInstanceObservation{ID: &guid})),
			},
			want: want{
				mg:  serviceInstance("managed", withSpace(spaceGUID), withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan}), withExternalName(guid), withStatus(v1alpha1.ServiceInstanceObservation{ID: &guid})),
				obs: managed.ExternalUpdate{},
				err: errors.Wrap(errBoom, errUpdate),
			},
			service: func() *fake.MockServiceInstance {
				m := &fake.MockServiceInstance{}
				m.On("UpdateManaged", guid).Return(
					"JOB123",
					nil,
				)
				m.On("Get", guid).Return(
					&fake.NewServiceInstance("managed").SetName(name).SetGUID(guid).SetServicePlan(servicePlan).ServiceInstance,
					nil,
				)
				m.On("GetManagedParameters", guid).Return(
					nil,
					nil, // no error
				)
				return m
			},
			job: func() *fake.MockJob {
				m := &fake.MockJob{}
				m.On("PollComplete").Return(errBoom)
				return m
			},
		},
		"DoesNotExist": {
			args: args{
				mg: serviceInstance("managed", withSpace(spaceGUID), withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan}), withExternalName(guid), withStatus(v1alpha1.ServiceInstanceObservation{ID: &guid})),
			},
			want: want{
				mg:  serviceInstance("managed", withSpace(spaceGUID), withServicePlan(v1alpha1.ServicePlanParameters{ID: &servicePlan}), withExternalName(guid), withStatus(v1alpha1.ServiceInstanceObservation{ID: &guid})),
				obs: managed.ExternalUpdate{},
				err: errors.Wrap(errBoom, errUpdate),
			},
			service: func() *fake.MockServiceInstance {
				m := &fake.MockServiceInstance{}
				m.On("UpdateManaged").Return(
					"JOB123",
					errBoom,
				)
				m.On("Get", guid).Return(
					fake.ServiceInstanceNil,
					errBoom,
				)
				return m
			},
			job: func() *fake.MockJob {
				m := &fake.MockJob{}
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
				serviceinstance: &serviceinstance.Client{
					ServiceInstance: tc.service(),
					Job:             tc.job(),
				},
			}
			obs, err := c.Update(context.Background(), tc.args.mg)

			if tc.want.err != nil && err != nil {
				// the case where our mock server returns error.
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
			if diff := cmp.Diff(tc.want.mg, tc.args.mg); diff != "" {
				t.Errorf("Update(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestJSONContain(t *testing.T) {
	type args struct {
		a string
		b string
	}
	type want struct {
		obs bool
	}
	cases := map[string]struct {
		args args
		want want
	}{
		"Empties": {
			args: args{
				a: "",
				b: "",
			},
			want: want{
				obs: true,
			},
		},
		"Empties2": {
			args: args{
				a: "",
				b: "{}",
			},
			want: want{
				obs: true,
			},
		},
		"ContainsEmpty": {
			args: args{
				a: `{"foo":"foo"}`,
				b: "",
			},
			want: want{
				obs: true,
			},
		},
		"EmptyContainsNone": {
			args: args{
				a: "",
				b: `{"foo":"foo"}`,
			},
			want: want{
				obs: false,
			},
		},
		"Equal": {
			args: args{
				a: `{"foo":"foo", "bar": 1}`,
				b: `{"foo":"foo", "bar": 1}`,
			},
			want: want{
				obs: true,
			},
		},
		"Superset": {
			args: args{
				a: `{ "bar": 1, "baz": "baz", "foo":"foo"}`,
				b: `{"foo":"foo",
				"bar": 1}`,
			},
			want: want{
				obs: true,
			},
		},
		"Subset": {
			args: args{
				a: `{"foo":"foo",
				 "bar": 1}`,
				b: `{"foo":"foo", "bar": 1, "baz": "baz"}`,
			},
			want: want{
				obs: false,
			},
		},
		"DiffValue": {
			args: args{
				a: `{"foo":"foo",
				 "bar": 1}`,
				b: `{"foo":"foo", "bar": 2}`,
			},
			want: want{
				obs: false,
			},
		},
	}
	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			obs := jsonContain([]byte(tc.args.a), []byte(tc.args.b))
			if diff := cmp.Diff(tc.want.obs, obs); diff != "" {
				t.Errorf("matchJSON(...): -want, +got:\n%s", diff)
			}
		})
	}
}
