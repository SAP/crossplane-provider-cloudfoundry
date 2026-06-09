package domain

import (
	"context"
	"testing"

	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/pkg/errors"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/fake"
)

var (
	errBoom      = errors.New("boom")
	resourceName = "my-domain"
	guid         = "2d8b0d04-d537-4e4e-8c6f-f09ca0e7f56f"
	name         = "sap.my-domain.com"

	healthyDomain = &cfresource.Domain{
		Resource: cfresource.Resource{
			GUID: guid,
		},
		Name:     name,
		Internal: false,
	}
)

type modifier func(*v1alpha1.Domain)

func withExternalName(name string) modifier {
	return func(r *v1alpha1.Domain) {
		r.Annotations[meta.AnnotationKeyExternalName] = name
	}
}

func withName(name string) modifier {
	return func(r *v1alpha1.Domain) {
		r.Spec.ForProvider.Name = name
	}
}

func withConditions(c ...xpv1.Condition) modifier {
	return func(i *v1alpha1.Domain) { i.Status.SetConditions(c...) }
}

func withID(id string) modifier {
	return func(r *v1alpha1.Domain) {
		r.Status.AtProvider.ID = ptr.To(id)
	}
}

func fakeDomain(m ...modifier) *v1alpha1.Domain {
	r := &v1alpha1.Domain{
		ObjectMeta: metav1.ObjectMeta{
			Name:        resourceName,
			Finalizers:  []string{},
			Annotations: map[string]string{},
		},
		Spec: v1alpha1.DomainSpec{
			ForProvider: v1alpha1.DomainParameters{},
		},
	}

	for _, rm := range m {
		rm(r)
	}
	return r
}

// mockDomainService implements DomainService interface for testing
type mockDomainService struct {
	FindDomainBySpecFunc func(ctx context.Context, spec v1alpha1.DomainParameters) (*cfresource.Domain, error)
	GetDomainByGUIDFunc  func(ctx context.Context, guid string) (*cfresource.Domain, error)
	CreateFunc           func(ctx context.Context, create *cfresource.DomainCreate) (*cfresource.Domain, error)
	UpdateFunc           func(ctx context.Context, guid string, update *cfresource.DomainUpdate) (*cfresource.Domain, error)
	DeleteFunc           func(ctx context.Context, guid string) (string, error)
}

func (m *mockDomainService) FindDomainBySpec(ctx context.Context, spec v1alpha1.DomainParameters) (*cfresource.Domain, error) {
	return m.FindDomainBySpecFunc(ctx, spec)
}

func (m *mockDomainService) GetDomainByGUID(ctx context.Context, guid string) (*cfresource.Domain, error) {
	return m.GetDomainByGUIDFunc(ctx, guid)
}

func (m *mockDomainService) Create(ctx context.Context, create *cfresource.DomainCreate) (*cfresource.Domain, error) {
	return m.CreateFunc(ctx, create)
}

func (m *mockDomainService) Update(ctx context.Context, guid string, update *cfresource.DomainUpdate) (*cfresource.Domain, error) {
	return m.UpdateFunc(ctx, guid, update)
}

func (m *mockDomainService) Delete(ctx context.Context, guid string) (string, error) {
	return m.DeleteFunc(ctx, guid)
}

func TestObserve(t *testing.T) {
	type service func() *mockDomainService
	type args struct {
		mg resource.Managed
	}

	type want struct {
		mg  *v1alpha1.Domain
		obs managed.ExternalObservation
		err error
	}

	cases := map[string]struct {
		args    args
		want    want
		service service
		kube    k8s.Client
	}{
		"ObserveNilProvider": {
			args: args{
				mg: nil,
			},
			want: want{
				obs: managed.ExternalObservation{},
				err: errors.New(errNotDomainKind),
			},
			service: func() *mockDomainService {
				return &mockDomainService{}
			},
		},
		"ObserveUnsetExternalNameSuccessful": {
			args: args{
				mg: fakeDomain(withName(name)),
			},
			want: want{
				mg:  fakeDomain(withName(name), withExternalName(guid), withConditions(xpv1.Available())),
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: false, ResourceLateInitialized: true},
				err: nil,
			},
			service: func() *mockDomainService {
				return &mockDomainService{
					FindDomainBySpecFunc: func(ctx context.Context, spec v1alpha1.DomainParameters) (*cfresource.Domain, error) {
						return healthyDomain, nil
					},
					GetDomainByGUIDFunc: func(ctx context.Context, guid string) (*cfresource.Domain, error) {
						return healthyDomain, nil
					},
				}
			},
		},
		"ObserveUnsetExternalNameNotFound": {
			args: args{
				mg: fakeDomain(withName(name)),
			},
			want: want{
				mg:  fakeDomain(withName(name)),
				obs: managed.ExternalObservation{ResourceExists: false},
				err: nil,
			},
			service: func() *mockDomainService {
				return &mockDomainService{
					FindDomainBySpecFunc: func(ctx context.Context, spec v1alpha1.DomainParameters) (*cfresource.Domain, error) {
						return nil, nil
					},
				}
			},
		},
		"ObserveUnsetExternalNameError": {
			args: args{
				mg: fakeDomain(withName(name)),
			},
			want: want{
				mg:  fakeDomain(withName(name)),
				obs: managed.ExternalObservation{},
				err: errors.Wrap(errBoom, errGet),
			},
			service: func() *mockDomainService {
				return &mockDomainService{
					FindDomainBySpecFunc: func(ctx context.Context, spec v1alpha1.DomainParameters) (*cfresource.Domain, error) {
						return nil, errBoom
					},
				}
			},
		},
		"ObserveSetExternalNameSuccessful": {
			args: args{
				mg: fakeDomain(withExternalName(guid), withName(name)),
			},
			want: want{
				mg:  fakeDomain(withExternalName(guid), withName(name), withConditions(xpv1.Available())),
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: false},
				err: nil,
			},
			service: func() *mockDomainService {
				return &mockDomainService{
					GetDomainByGUIDFunc: func(ctx context.Context, guid string) (*cfresource.Domain, error) {
						return healthyDomain, nil
					},
				}
			},
		},
		"ObserveSetExternalNameNotFound": {
			args: args{
				mg: fakeDomain(withExternalName(guid)),
			},
			want: want{
				mg:  fakeDomain(withExternalName(guid)),
				obs: managed.ExternalObservation{ResourceExists: false},
				err: nil,
			},
			service: func() *mockDomainService {
				return &mockDomainService{
					GetDomainByGUIDFunc: func(ctx context.Context, guid string) (*cfresource.Domain, error) {
						return nil, fake.ErrNoResultReturned
					},
				}
			},
		},
		"ObserveSetExternalNameInvalidFormat": {
			args: args{
				mg: fakeDomain(withExternalName("not-a-valid-guid")),
			},
			want: want{
				mg:  fakeDomain(withExternalName("not-a-valid-guid")),
				obs: managed.ExternalObservation{},
				err: errors.New("external-name 'not-a-valid-guid' is not a valid GUID format"),
			},
			service: func() *mockDomainService {
				return &mockDomainService{}
			},
		},
		"ObserveError": {
			args: args{
				mg: fakeDomain(withExternalName(guid)),
			},
			want: want{
				mg:  fakeDomain(withExternalName(guid)),
				obs: managed.ExternalObservation{},
				err: errors.Wrap(errBoom, errGet),
			},
			service: func() *mockDomainService {
				return &mockDomainService{
					GetDomainByGUIDFunc: func(ctx context.Context, guid string) (*cfresource.Domain, error) {
						return nil, errBoom
					},
				}
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			c := &external{
				kube: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(nil),
				},
				job:    nil,
				client: tc.service(),
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
			if tc.want.mg != nil {
				if diff := cmp.Diff(tc.want.mg, tc.args.mg, cmp.Options{cmpopts.IgnoreFields(v1alpha1.Domain{}, "Status.AtProvider")}); diff != "" {
					t.Errorf("Observe(-want, +got):\n%s", diff)
				}
			}
		})
	}
}

func TestCreate(t *testing.T) {
	type service func() *mockDomainService
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
		kube    k8s.Client
	}{
		"CreateSuccessful": {
			args: args{
				mg: fakeDomain(withName(name)),
			},
			want: want{
				mg: fakeDomain(withName(name), withExternalName(guid), withConditions(xpv1.Creating())),
				obs: managed.ExternalCreation{
					ConnectionDetails: managed.ConnectionDetails{},
				},
				err: nil,
			},
			service: func() *mockDomainService {
				return &mockDomainService{
					CreateFunc: func(ctx context.Context, create *cfresource.DomainCreate) (*cfresource.Domain, error) {
						return healthyDomain, nil
					},
				}
			},
		},
		"CreateError": {
			args: args{
				mg: fakeDomain(withName(name)),
			},
			want: want{
				mg:  fakeDomain(withName(name), withConditions(xpv1.Creating())),
				obs: managed.ExternalCreation{},
				err: errors.Wrap(errBoom, errCreate),
			},
			service: func() *mockDomainService {
				return &mockDomainService{
					CreateFunc: func(ctx context.Context, create *cfresource.DomainCreate) (*cfresource.Domain, error) {
						return nil, errBoom
					},
				}
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
				job:    nil,
				client: tc.service(),
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
			if tc.want.mg != nil {
				if diff := cmp.Diff(tc.want.mg, tc.args.mg, cmp.Options{cmpopts.IgnoreFields(v1alpha1.Domain{}, "Status.AtProvider")}); diff != "" {
					t.Errorf("Create(-want, +got):\n%s", diff)
				}
			}
		})
	}
}

func TestUpdate(t *testing.T) {
	type service func() *mockDomainService
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
		kube    k8s.Client
	}{
		"UpdateSuccessful": {
			args: args{
				mg: fakeDomain(withExternalName(guid), withName(name), withID(guid)),
			},
			want: want{
				mg:  fakeDomain(withExternalName(guid), withName(name), withID(guid)),
				obs: managed.ExternalUpdate{},
				err: nil,
			},
			service: func() *mockDomainService {
				return &mockDomainService{
					UpdateFunc: func(ctx context.Context, guid string, update *cfresource.DomainUpdate) (*cfresource.Domain, error) {
						return healthyDomain, nil
					},
				}
			},
		},
		"UpdateEmptyExternalName": {
			args: args{
				mg: fakeDomain(withName(name)),
			},
			want: want{
				mg:  fakeDomain(withName(name)),
				obs: managed.ExternalUpdate{},
				err: nil,
			},
			service: func() *mockDomainService {
				return &mockDomainService{}
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
				job:    nil,
				client: tc.service(),
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
			if tc.want.mg != nil {
				if diff := cmp.Diff(tc.want.mg, tc.args.mg, cmp.Options{cmpopts.IgnoreFields(v1alpha1.Domain{}, "Status.AtProvider")}); diff != "" {
					t.Errorf("Update(-want, +got):\n%s", diff)
				}
			}
		})
	}
}

func TestDelete(t *testing.T) {
	type service func() *mockDomainService
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
		"DeleteSuccessful": {
			args: args{
				mg: fakeDomain(withExternalName(guid), withID(guid)),
			},
			want: want{
				mg:  fakeDomain(withExternalName(guid), withID(guid), withConditions(xpv1.Deleting())),
				obs: managed.ExternalDelete{},
				err: nil,
			},
			service: func() *mockDomainService {
				return &mockDomainService{
					DeleteFunc: func(ctx context.Context, guid string) (string, error) {
						return "job-guid-123", nil
					},
				}
			},
		},
		"DeleteNotFound": {
			args: args{
				mg: fakeDomain(withExternalName(guid), withID(guid)),
			},
			want: want{
				mg:  fakeDomain(withExternalName(guid), withID(guid), withConditions(xpv1.Deleting())),
				obs: managed.ExternalDelete{},
				err: nil,
			},
			service: func() *mockDomainService {
				return &mockDomainService{
					DeleteFunc: func(ctx context.Context, guid string) (string, error) {
						return "", fake.ErrNoResultReturned
					},
				}
			},
		},
		"DeleteEmptyExternalName": {
			args: args{
				mg: fakeDomain(withName(name)),
			},
			want: want{
				mg:  fakeDomain(withName(name), withConditions(xpv1.Deleting())),
				obs: managed.ExternalDelete{},
				err: nil,
			},
			service: func() *mockDomainService {
				return &mockDomainService{}
			},
		},
		"DeleteError": {
			args: args{
				mg: fakeDomain(withExternalName(guid), withID(guid)),
			},
			want: want{
				mg:  fakeDomain(withExternalName(guid), withID(guid), withConditions(xpv1.Deleting())),
				obs: managed.ExternalDelete{},
				err: errors.Wrap(errBoom, errDelete),
			},
			service: func() *mockDomainService {
				return &mockDomainService{
					DeleteFunc: func(ctx context.Context, guid string) (string, error) {
						return "", errBoom
					},
				}
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			mockJob := &fake.MockJob{}
			mockJob.On("PollComplete").Return(nil)

			c := &external{
				kube: &test.MockClient{
					MockDelete:       test.NewMockDeleteFn(nil),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				job:    mockJob,
				client: tc.service(),
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
			if tc.want.mg != nil {
				if diff := cmp.Diff(tc.want.mg, tc.args.mg, cmp.Options{cmpopts.IgnoreFields(v1alpha1.Domain{}, "Status.AtProvider")}); diff != "" {
					t.Errorf("Delete(-want, +got):\n%s", diff)
				}
			}
		})
	}
}
