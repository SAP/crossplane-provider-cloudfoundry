package route

import (
	"context"
	"testing"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/fake"
)

// Mock mocks RouteService interface
type Mock struct {
	mock.Mock
}

func (m *Mock) FindRouteBySpec(ctx context.Context, forProvider v1alpha1.RouteParameters) (*v1alpha1.RouteObservation, bool, error) {
	args := m.Called(forProvider)
	return args.Get(0).(*v1alpha1.RouteObservation), args.Bool(1), args.Error(2)
}

func (m *Mock) GetRouteByGUID(ctx context.Context, guid string) (*v1alpha1.RouteObservation, bool, error) {
	args := m.Called(guid)
	return args.Get(0).(*v1alpha1.RouteObservation), args.Bool(1), args.Error(2)
}

func (m *Mock) Create(ctx context.Context, forProvider v1alpha1.RouteParameters) (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *Mock) Update(ctx context.Context, guid string, forProvider v1alpha1.RouteParameters) error {
	args := m.Called()
	return args.Error(0)
}

func (m *Mock) Delete(ctx context.Context, guid string) (string, error) {
	args := m.Called(guid)
	return args.String(0), args.Error(1)
}

var (
	spaceGUID  = "11fd5b0b-4f3b-4b1b-8b3d-3b5f7b4b3b4b"
	domainGUID = "22fd5b0b-4f3b-4b1b-8b3d-3b5f7b4b3b4b"
	guid       = "33fd5b0b-4f3b-4b1b-8b3d-3b5f7b4b3b4b"
	name       = "test-route"
	errBoom    = errors.New("boom")

	nilObservation *v1alpha1.RouteObservation
)

type modifier func(*v1alpha1.Route)

func withExternalName(externalName string) modifier {
	return func(r *v1alpha1.Route) {
		r.Annotations[meta.AnnotationKeyExternalName] = externalName
	}
}

func withHost(host string) modifier {
	return func(r *v1alpha1.Route) {
		r.Spec.ForProvider.Host = &host
	}
}

func withConditions(c ...xpv1.Condition) modifier {
	return func(r *v1alpha1.Route) { r.Status.SetConditions(c...) }
}

func withDestinations(destinations []v1alpha1.RouteDestination) modifier {
	return func(r *v1alpha1.Route) {
		r.Status.AtProvider.Destinations = destinations
	}
}

func fakeRoute(m ...modifier) *v1alpha1.Route {
	r := &v1alpha1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Finalizers:  []string{},
			Annotations: map[string]string{},
		},
		Spec: v1alpha1.RouteSpec{
			ForProvider: v1alpha1.RouteParameters{
				SpaceReference:  v1alpha1.SpaceReference{Space: &spaceGUID},
				DomainReference: v1alpha1.DomainReference{Domain: &domainGUID},
			},
		},
	}

	for _, rm := range m {
		rm(r)
	}
	return r
}

func fakeRouteObservation(id string) *v1alpha1.RouteObservation {
	res := v1alpha1.Resource{
		GUID: id,
	}
	r := &v1alpha1.RouteObservation{
		Resource: res,
	}
	return r
}

func TestObserve(t *testing.T) {
	type service func() *Mock
	type args struct {
		mg resource.Managed
	}

	type want struct {
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
				obs: managed.ExternalObservation{},
				err: errors.New(errNotRoute),
			},
			service: func() *Mock {
				m := &Mock{}
				return m
			},
		},
		"UnsetExternalNameSuccessful": {
			args: args{
				mg: fakeRoute(withHost(name)),
			},
			want: want{
				obs: managed.ExternalObservation{
					ResourceExists:          true,
					ResourceUpToDate:        true,
					ResourceLateInitialized: true,
				},
				err: nil,
			},
			service: func() *Mock {
				m := &Mock{}
				m.On("FindRouteBySpec", fakeRoute(withHost(name)).Spec.ForProvider).Return(
					fakeRouteObservation(guid), true, nil,
				)
				m.On("GetRouteByGUID", guid).Return(
					fakeRouteObservation(guid), true, nil,
				)
				return m
			},
		},
		"UnsetExternalNameNotFound": {
			args: args{
				mg: fakeRoute(withHost(name)),
			},
			want: want{
				obs: managed.ExternalObservation{ResourceExists: false},
				err: nil,
			},
			service: func() *Mock {
				m := &Mock{}
				m.On("FindRouteBySpec", fakeRoute(withHost(name)).Spec.ForProvider).Return(
					nilObservation, false, nil,
				)
				return m
			},
		},
		"SetExternalNameSuccessful": {
			args: args{
				mg: fakeRoute(
					withExternalName(guid),
					withHost(name),
				),
			},
			want: want{
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
				err: nil,
			},
			service: func() *Mock {
				m := &Mock{}
				m.On("GetRouteByGUID", guid).Return(
					fakeRouteObservation(guid), true, nil,
				)
				return m
			},
		},
		"SetExternalNameNotFound": {
			args: args{
				mg: fakeRoute(withExternalName(guid)),
			},
			want: want{
				obs: managed.ExternalObservation{ResourceExists: false},
				err: nil,
			},
			service: func() *Mock {
				m := &Mock{}
				m.On("GetRouteByGUID", guid).Return(
					nilObservation, false, nil,
				)
				return m
			},
		},
		"SetExternalNameInvalidFormat": {
			args: args{
				mg: fakeRoute(withExternalName("not-a-valid-guid")),
			},
			want: want{
				obs: managed.ExternalObservation{},
				err: errors.New("external-name 'not-a-valid-guid' is not a valid GUID format"),
			},
			service: func() *Mock {
				return &Mock{}
			},
		},
		"Error": {
			args: args{
				mg: fakeRoute(withExternalName(guid)),
			},
			want: want{
				obs: managed.ExternalObservation{},
				err: errors.Wrap(errBoom, errGet),
			},
			service: func() *Mock {
				m := &Mock{}
				m.On("GetRouteByGUID", guid).Return(
					nilObservation, false, errBoom,
				)
				return m
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			c := &external{
				kube: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(nil),
				},
				RouteService: tc.service(),
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
	type service func() *Mock
	type args struct {
		mg resource.Managed
	}

	type want struct {
		obs managed.ExternalCreation
		err error
	}

	cases := map[string]struct {
		args    args
		want    want
		service service
		kube    k8s.Client
	}{
		"Successful": {
			args: args{
				mg: fakeRoute(),
			},
			want: want{
				obs: managed.ExternalCreation{ConnectionDetails: managed.ConnectionDetails{}},
				err: nil,
			},
			service: func() *Mock {
				m := &Mock{}
				m.On("Create").Return(guid, nil)
				return m
			},
		},
		"AlreadyExist": {
			args: args{
				mg: fakeRoute(),
			},
			want: want{
				obs: managed.ExternalCreation{},
				err: errors.Wrap(errBoom, errCreate),
			},
			service: func() *Mock {
				m := &Mock{}
				m.On("Create").Return("", errBoom)
				return m
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			c := &external{
				kube: &test.MockClient{
					MockUpdate:       test.NewMockUpdateFn(nil),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				RouteService: tc.service(),
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

func TestDelete(t *testing.T) {
	type service func() *Mock
	type args struct {
		mg resource.Managed
	}

	type want struct {
		mg  resource.Managed
		obs managed.ExternalDelete
		err error
	}

	cases := map[string]struct {
		args    args
		want    want
		service service
		kube    k8s.Client
	}{
		"SuccessfulDelete": {
			args: args{
				mg: fakeRoute(withExternalName(guid)),
			},
			want: want{
				mg:  fakeRoute(withExternalName(guid), withConditions(xpv1.Deleting())),
				obs: managed.ExternalDelete{},
				err: nil,
			},
			service: func() *Mock {
				m := &Mock{}
				m.On("Delete", guid).Return("job-guid-123", nil)
				return m
			},
		},
		"404NotFound": {
			args: args{
				mg: fakeRoute(withExternalName(guid)),
			},
			want: want{
				mg:  fakeRoute(withExternalName(guid), withConditions(xpv1.Deleting())),
				obs: managed.ExternalDelete{},
				err: nil,
			},
			service: func() *Mock {
				m := &Mock{}
				m.On("Delete", guid).Return("", nil)
				return m
			},
		},
		"Error": {
			args: args{
				mg: fakeRoute(withExternalName(guid)),
			},
			want: want{
				mg:  fakeRoute(withExternalName(guid), withConditions(xpv1.Deleting())),
				obs: managed.ExternalDelete{},
				err: errors.Wrap(errBoom, errDelete),
			},
			service: func() *Mock {
				m := &Mock{}
				m.On("Delete", guid).Return("", errBoom)
				return m
			},
		},
		"EmptyExternalName": {
			args: args{
				mg: fakeRoute(),
			},
			want: want{
				mg:  fakeRoute(withConditions(xpv1.Deleting())),
				obs: managed.ExternalDelete{},
				err: nil,
			},
			service: func() *Mock {
				m := &Mock{}
				return m
			},
		},
		"ActiveBindings": {
			args: args{
				mg: fakeRoute(withExternalName(guid), withDestinations([]v1alpha1.RouteDestination{{GUID: "dest-guid"}})),
			},
			want: want{
				mg:  fakeRoute(withExternalName(guid), withDestinations([]v1alpha1.RouteDestination{{GUID: "dest-guid"}})),
				obs: managed.ExternalDelete{},
				err: errors.New(errActiveBinding),
			},
			service: func() *Mock {
				m := &Mock{}
				return m
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			mockJob := &fake.MockJob{}
			mockJob.On("PollComplete").Return(nil)

			c := &external{
				kube: &test.MockClient{
					MockDelete: test.NewMockDeleteFn(nil),
				},
				job:          mockJob,
				RouteService: tc.service(),
			}

			obs, err := c.Delete(context.Background(), tc.args.mg)

			if tc.want.err != nil && err != nil {
				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("Delete(...): want error string != got error string:\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(tc.want.err, err); diff != "" {
					t.Errorf("Delete(...): want error != got error:\n%s", diff)
				}
			}
			if diff := cmp.Diff(tc.want.obs, obs); diff != "" {
				t.Errorf("Delete(...): -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.mg, tc.args.mg); diff != "" {
				t.Errorf("Delete(...): -want, +got:\n%s", diff)
			}
		})
	}
}
