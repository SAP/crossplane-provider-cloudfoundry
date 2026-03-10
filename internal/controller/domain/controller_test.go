package domain

import (
	"context"
	"testing"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/fake"
)

var (
	errBoom = errors.New("boom")
	name    = "sap.my-domain.com"
	guid    = "2d8b0d04-d537-4e4e-8c6f-f09ca0e7f56f"
)

type modifier func(*v1alpha1.Domain)

func withExternalName(name string) modifier {
	return func(r *v1alpha1.Domain) {
		r.ObjectMeta.Annotations[meta.AnnotationKeyExternalName] = name
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
		r.Status.AtProvider.ID = &id
	}
}

func fakeDomain(m ...modifier) *v1alpha1.Domain {
	r := &v1alpha1.Domain{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
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

func TestObserve(t *testing.T) {
	type service func() *fake.MockDomain
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
		"Error if cr is not the right kind": {
			args: args{
				mg: nil,
			},
			want: want{
				mg:  nil,
				obs: managed.ExternalObservation{ResourceExists: false},
				err: errors.New(errNotDomainKind),
			},
			service: func() *fake.MockDomain {
				m := &fake.MockDomain{}
				return m
			},
		},
		// This tests whether the external API is reachable
		"Error when external API is not working": {
			args: args{
				mg: fakeDomain(withExternalName(guid)),
			},
			want: want{
				mg:  fakeDomain(withExternalName(guid)),
				obs: managed.ExternalObservation{},
				err: errors.Wrap(errBoom, errGetResource),
			},
			service: func() *fake.MockDomain {
				m := &fake.MockDomain{}
				m.On("Get", guid).Return(
					fake.DomainNil,
					errBoom,
				)
				m.On("Single").Return(
					fake.DomainNil,
					errBoom,
				)
				return m
			},
		},
		"NotFound if Domain with guid is not found. Match by name should not be called": {
			args: args{
				mg: fakeDomain(withExternalName(guid)),
			},
			want: want{
				mg:  fakeDomain(withExternalName(guid)),
				obs: managed.ExternalObservation{ResourceExists: false, ResourceLateInitialized: false},
				err: nil,
			},
			service: func() *fake.MockDomain {
				m := &fake.MockDomain{}
				m.On("Get", guid).Return(
					fake.DomainNil,
					fake.ErrNoResultReturned,
				)
				m.On("Single").Return( // this should not be called
					&fake.NewDomain().SetName(name).SetGUID(guid).Domain,
					nil,
				)
				return m
			},
			kube: &test.MockClient{},
		},
		"NotFound by uuid is not provided and Domain with name is not found": {
			args: args{
				mg: fakeDomain(withName(name), withExternalName("not-a-uuid")),
			},
			want: want{
				mg: fakeDomain(withName(name), withExternalName(guid)),
				obs: managed.ExternalObservation{
					ResourceExists: false,
				},
				err: nil,
			},
			service: func() *fake.MockDomain {
				m := &fake.MockDomain{}
				m.On("Get", "").Return( // this should be called
					fake.DomainNil,
					errBoom,
				)
				m.On("Single").Return(
					fake.DomainNil,
					fake.ErrNoResultReturned,
				)
				return m
			},
		},
		"Successful when Domain with guid is found": {
			args: args{
				mg: fakeDomain(
					withExternalName(guid),
					withName(name),
				),
			},
			want: want{
				mg: fakeDomain(
					withExternalName(guid),
					withName(name),
				),
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
				err: nil,
			},
			service: func() *fake.MockDomain {
				m := &fake.MockDomain{}

				m.On("Get", guid).Return(
					&fake.NewDomain().SetName(name).SetGUID(guid).Domain,
					nil,
				)
				m.On("Single").Return(
					&fake.NewDomain().SetName(name).SetGUID(guid).Domain,
					nil,
				)
				return m
			},
		},
		"Successful when guid is not provided and Domain with name is found ": {
			args: args{
				mg: fakeDomain(withName(name)),
			},
			want: want{
				mg: fakeDomain(withName(name), withExternalName(guid)),
				obs: managed.ExternalObservation{
					ResourceExists:          true,
					ResourceUpToDate:        true,
					ResourceLateInitialized: true,
				},
				err: nil,
			},
			service: func() *fake.MockDomain {
				m := &fake.MockDomain{}
				m.On("Get", "").Return(
					fake.DomainNil,
					fake.ErrNoResultReturned,
				)
				m.On("Single").Return(
					&fake.NewDomain().SetName(name).SetGUID(guid).Domain,
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
				client: tc.service(),
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

func TestCreate(t *testing.T) {
	type service func() *fake.MockDomain
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
		"Successful": {
			args: args{
				mg: fakeDomain(withExternalName(guid)),
			},
			want: want{
				mg: fakeDomain(withExternalName(guid),
					withConditions(xpv1.Creating())),
				obs: managed.ExternalCreation{ConnectionDetails: managed.ConnectionDetails{}},
				err: nil,
			},
			service: func() *fake.MockDomain {
				m := &fake.MockDomain{}
				m.On("Create").Return(
					&fake.NewDomain().SetName(name).SetGUID(guid).Domain,
					nil,
				)
				m.On("Single").Return(
					&fake.NewDomain().SetName(name).SetGUID(guid).Domain,
					nil,
				)
				return m
			},
		},
		"AlreadyExist": {
			args: args{
				mg: fakeDomain(withExternalName(guid)),
			},
			want: want{
				mg: fakeDomain(withExternalName(guid),
					withConditions(xpv1.Creating())),
				obs: managed.ExternalCreation{},
				err: errors.Wrap(errBoom, errCreate),
			},
			service: func() *fake.MockDomain {
				m := &fake.MockDomain{}
				m.On("Create").Return(
					&fake.NewDomain().SetName(name).SetGUID(guid).Domain,
					errBoom,
				)
				m.On("Single").Return(
					&fake.NewDomain().SetName(name).SetGUID(guid).Domain,
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
					MockUpdate:       test.NewMockUpdateFn(nil),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				client: tc.service(),
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
	type service func() *fake.MockDomain
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
		kube    k8s.Client
	}{
		"SuccessfulDelete": {
			args: args{
				mg: fakeDomain(withExternalName(guid), withID(guid)),
			},
			want: want{
				mg:  fakeDomain(withExternalName(guid), withID(guid)),
				err: nil,
			},
			service: func() *fake.MockDomain {
				m := &fake.MockDomain{}
				m.On("Delete").Return(
					"",
					nil,
				)
				return m
			},
		},
		"IDNotSet": {
			args: args{
				mg: fakeDomain(withExternalName(guid)),
			},
			want: want{
				mg:  fakeDomain(withExternalName(guid)),
				err: errors.New(errDelete),
			},
			service: func() *fake.MockDomain {
				m := &fake.MockDomain{}
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
				client: tc.service(),
			}

			_, err := c.Delete(context.Background(), tc.args.mg)

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
		})
	}
}
