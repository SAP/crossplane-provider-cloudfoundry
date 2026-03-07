package route

import (
	"context"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
)

// Mock mocks RouteService interface
type Mock struct {
	mock.Mock
}

func (m *Mock) GetByIDOrSpec(ctx context.Context, guid string, forProvider v1alpha1.RouteParameters) (*v1alpha1.RouteObservation, error) {
	args := m.Called(guid)
	return args.Get(0).(*v1alpha1.RouteObservation), args.Error(1)
}

func (m *Mock) Create(ctx context.Context, forProvider v1alpha1.RouteParameters) (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *Mock) Update(ctx context.Context, guid string, forProvider v1alpha1.RouteParameters) error {
	args := m.Called()
	return args.Error(0)
}

func (m *Mock) Delete(ctx context.Context, guid string) error {
	args := m.Called()
	return args.Error(0)
}

var (
	spaceGUID      = "11fd5b0b-4f3b-4b1b-8b3d-3b5f7b4b3b4b"
	domainGUID     = "22fd5b0b-4f3b-4b1b-8b3d-3b5f7b4b3b4b"
	guid           = "33fd5b0b-4f3b-4b1b-8b3d-3b5f7b4b3b4b"
	name           = "test-route"
	errBoom        = errors.New("boom")
	nilObservation *v1alpha1.RouteObservation
)

type modifier func(*v1alpha1.Route)

func withExternalName(guid string) modifier {
	return func(r *v1alpha1.Route) {
		r.ObjectMeta.Annotations[meta.AnnotationKeyExternalName] = guid
	}
}

func withHost(host string) modifier {
	return func(r *v1alpha1.Route) {
		r.Spec.ForProvider.Host = &host
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
		mg  *v1alpha1.Route
		obs managed.ExternalObservation
		err error
	}

	cases := map[string]struct {
		args    args
		want    want
		service service
		kube    k8s.Client
	}{
		"Error if mg is not the right kind": {
			args: args{
				mg: nil,
			},
			want: want{
				mg:  nil,
				obs: managed.ExternalObservation{ResourceExists: false},
				err: errors.New(errNotRoute),
			},
			service: func() *Mock {
				m := &Mock{}
				return m
			},
		},
		// This tests whether the external API is reachable
		"Error when external API is not working": {
			args: args{
				mg: fakeRoute(withExternalName(guid)),
			},
			want: want{
				mg:  fakeRoute(withExternalName(guid)),
				obs: managed.ExternalObservation{},
				err: errors.Wrap(errBoom, errGet),
			},
			service: func() *Mock {
				m := &Mock{}
				m.On("GetByIDOrSpec", guid).Return(
					nilObservation,
					errBoom,
				)
				return m
			},
		},
		"NotFound (guid) with nil observation ": {
			args: args{
				mg: fakeRoute(withExternalName(guid)),
			},
			want: want{
				mg:  fakeRoute(withExternalName(guid)),
				obs: managed.ExternalObservation{ResourceExists: false, ResourceLateInitialized: false},
				err: nil,
			},
			service: func() *Mock {
				m := &Mock{}
				m.On("GetByIDOrSpec", guid).Return(
					nilObservation,
					nil,
				)
				return m
			},
			kube: &test.MockClient{},
		},
		"NotFound if nil observation is returned": {
			args: args{
				mg: fakeRoute(withHost(name)),
			},
			want: want{
				mg: fakeRoute(withHost(name), withExternalName(guid)),
				obs: managed.ExternalObservation{
					ResourceExists: false,
				},
				err: nil,
			},
			service: func() *Mock {
				m := &Mock{}
				m.On("GetByIDOrSpec", "").Return( // this should be called
					nilObservation,
					nil,
				)
				return m
			},
		},
		"Found with observation is returned": {
			args: args{
				mg: fakeRoute(
					withExternalName(guid),
					withHost(name),
				),
			},
			want: want{
				mg: fakeRoute(
					withExternalName(guid),
					withHost(name),
				),
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
				err: nil,
			},
			service: func() *Mock {
				m := &Mock{}

				m.On("GetByIDOrSpec", guid).Return(
					fakeRouteObservation(guid),
					nil,
				)
				return m
			},
		},
		"Adopt and set external-name ": {
			args: args{
				mg: fakeRoute(withHost(name)),
			},
			want: want{
				mg: fakeRoute(withHost(name), withExternalName(guid)),
				obs: managed.ExternalObservation{
					ResourceExists:          true,
					ResourceUpToDate:        true,
					ResourceLateInitialized: true,
				},
				err: nil,
			},
			service: func() *Mock {
				m := &Mock{}
				m.On("GetByIDOrSpec", "").Return(
					fakeRouteObservation(guid),
					nil,
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

			var Domain *v1alpha1.Domain
			if tc.args.mg != nil {
				Domain, _ = tc.args.mg.(*v1alpha1.Domain)
			}

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
			if Domain != nil && tc.want.mg != nil {

				if diff := cmp.Diff(Domain.Spec, tc.want.mg.Spec); diff != "" {
					t.Errorf("Observe(...): -want, +got:\n%s", diff)
				}
			}
		})
	}
}
