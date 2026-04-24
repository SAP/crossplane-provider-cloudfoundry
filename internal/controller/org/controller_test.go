package org

import (
	"context"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/google/go-cmp/cmp"
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
	errBoom = errors.New("boom")
	name    = "my-org"
	guid    = "2d8b0d04-d537-4e4e-8c6f-f09ca0e7f56f"
)

type modifier func(*v1alpha1.Organization)

func withExternalName(name string) modifier {
	return func(r *v1alpha1.Organization) {
		r.Annotations[meta.AnnotationKeyExternalName] = name
	}
}

func withName(name string) modifier {
	return func(r *v1alpha1.Organization) {
		r.Spec.ForProvider.Name = name
	}
}

func fakeOrg(m ...modifier) *v1alpha1.Organization {
	r := &v1alpha1.Organization{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Finalizers:  []string{},
			Annotations: map[string]string{},
		},
		Spec: v1alpha1.OrgSpec{
			ForProvider: v1alpha1.OrgParameters{
				Suspended: ptr.To(false),
			},
		},
	}

	for _, rm := range m {
		rm(r)
	}
	return r
}

func TestObserve(t *testing.T) {
	type service func() *fake.MockOrganization
	type args struct {
		mg resource.Managed
	}

	type want struct {
		mg  *v1alpha1.Organization
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
				mg:  nil,
				obs: managed.ExternalObservation{},
				err: errors.New(errNotOrgKind),
			},
			service: func() *fake.MockOrganization {
				return &fake.MockOrganization{}
			},
		},
		"Boom!": {
			args: args{
				mg: fakeOrg(withExternalName(guid), withName(name)),
			},
			want: want{
				mg:  fakeOrg(withExternalName(guid), withName(name)),
				obs: managed.ExternalObservation{},
				err: errors.Wrap(errBoom, errGetResource),
			},
			service: func() *fake.MockOrganization {
				m := &fake.MockOrganization{}
				// GetOrgByGUID calls Get, which returns error
				m.On("Get", guid).Return(
					fake.OrganizationNil,
					errBoom,
				)
				return m
			},
		},
		"UnsetExternalNameSuccessful": {
			args: args{
				mg: fakeOrg(withName(name)),
			},
			want: want{
				mg: fakeOrg(withName(name), withExternalName(guid)),
				obs: managed.ExternalObservation{
					ResourceExists:          true,
					ResourceUpToDate:        true,
					ResourceLateInitialized: true,
				},
				err: nil,
			},
			service: func() *fake.MockOrganization {
				m := &fake.MockOrganization{}
				// FindOrgBySpec calls Single to find by name
				m.On("Single").Return(
					&fake.NewOrganization().SetName(name).SetGUID(guid).Organization,
					nil,
				).Once()
				// GetOrgByGUID calls Get with the discovered GUID
				m.On("Get", guid).Return(
					&fake.NewOrganization().SetName(name).SetGUID(guid).Organization,
					nil,
				)
				return m
			},
		},
		"UnsetExternalNameNotFound": {
			args: args{
				mg: fakeOrg(withName(name)),
			},
			want: want{
				mg:  fakeOrg(withName(name)),
				obs: managed.ExternalObservation{ResourceExists: false},
				err: nil,
			},
			service: func() *fake.MockOrganization {
				m := &fake.MockOrganization{}
				// FindOrgBySpec calls Single, not-found returns nil, nil
				m.On("Single").Return(
					fake.OrganizationNil,
					nil,
				)
				return m
			},
		},
		"UnsetExternalNameError": {
			args: args{
				mg: fakeOrg(withName(name)),
			},
			want: want{
				mg:  fakeOrg(withName(name)),
				obs: managed.ExternalObservation{},
				err: errors.Wrap(errBoom, errGetResource),
			},
			service: func() *fake.MockOrganization {
				m := &fake.MockOrganization{}
				// FindOrgBySpec calls Single, which returns error
				m.On("Single").Return(
					fake.OrganizationNil,
					errBoom,
				)
				return m
			},
		},
		"SetExternalNameSuccessful": {
			args: args{
				mg: fakeOrg(withExternalName(guid), withName(name)),
			},
			want: want{
				mg: fakeOrg(withExternalName(guid), withName(name)),
				obs: managed.ExternalObservation{
					ResourceExists:   true,
					ResourceUpToDate: true,
				},
				err: nil,
			},
			service: func() *fake.MockOrganization {
				m := &fake.MockOrganization{}
				// GetOrgByGUID calls Get with the GUID
				m.On("Get", guid).Return(
					&fake.NewOrganization().SetName(name).SetGUID(guid).Organization,
					nil,
				)
				return m
			},
		},
		"SetExternalNameNotFound": {
			args: args{
				mg: fakeOrg(withExternalName(guid)),
			},
			want: want{
				mg:  fakeOrg(withExternalName(guid)),
				obs: managed.ExternalObservation{ResourceExists: false},
				err: nil,
			},
			service: func() *fake.MockOrganization {
				m := &fake.MockOrganization{}
				m.On("Get", guid).Return(
					fake.OrganizationNil,
					fake.ErrNoResultReturned,
				)
				return m
			},
		},
		"SetExternalNameInvalidFormat": {
			args: args{
				mg: fakeOrg(withName(name), withExternalName("not-valid")),
			},
			want: want{
				mg:  fakeOrg(withName(name), withExternalName("not-valid")),
				obs: managed.ExternalObservation{},
				err: errors.New("external-name 'not-valid' is not a valid GUID format"),
			},
			service: func() *fake.MockOrganization {
				return &fake.MockOrganization{}
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

			var org *v1alpha1.Organization
			if tc.args.mg != nil {
				org, _ = tc.args.mg.(*v1alpha1.Organization)
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
			if org != nil && tc.want.mg != nil {
				if diff := cmp.Diff(org.Spec, tc.want.mg.Spec); diff != "" {
					t.Errorf("Observe(...): -want, +got:\n%s", diff)
				}
				if diff := cmp.Diff(org.Annotations, tc.want.mg.Annotations); diff != "" {
					t.Errorf("Observe(...): -want, +got:\n%s", diff)
				}
			}
		})
	}
}

func TestCreate(t *testing.T) {
	type service func() *fake.MockOrganization
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
				mg: fakeOrg(withExternalName(guid)),
			},
			want: want{
				mg:  fakeOrg(withExternalName(guid)),
				obs: managed.ExternalCreation{ConnectionDetails: managed.ConnectionDetails{}},
				err: nil,
			},
			service: func() *fake.MockOrganization {
				m := &fake.MockOrganization{}
				m.On("Create").Return(
					&fake.NewOrganization().SetName(name).SetGUID(guid).Organization,
					nil,
				)
				m.On("Single").Return(
					&fake.NewOrganization().SetName(name).SetGUID(guid).Organization,
					nil,
				)
				return m
			},
		},
		"AlreadyExist": {
			args: args{
				mg: fakeOrg(withExternalName(guid)),
			},
			want: want{
				mg:  fakeOrg(withExternalName(guid)),
				obs: managed.ExternalCreation{},
				err: errors.Wrap(errBoom, errCreate),
			},
			service: func() *fake.MockOrganization {
				m := &fake.MockOrganization{}
				m.On("Create").Return(
					&fake.NewOrganization().SetName(name).SetGUID(guid).Organization,
					errBoom,
				)
				m.On("Single").Return(
					&fake.NewOrganization().SetName(name).SetGUID(guid).Organization,
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

func TestDelete(t *testing.T) {
	type service func() *fake.MockOrganization
	type args struct {
		mg resource.Managed
	}

	type want struct {
		del managed.ExternalDelete
		err error
	}

	cases := map[string]struct {
		args    args
		want    want
		service service
	}{
		"Successful": {
			args: args{
				mg: fakeOrg(withExternalName(guid)),
			},
			want: want{
				del: managed.ExternalDelete{},
				err: nil,
			},
			service: func() *fake.MockOrganization {
				return &fake.MockOrganization{}
			},
		},
		"EmptyExternalName": {
			args: args{
				mg: fakeOrg(withName(name)),
			},
			want: want{
				del: managed.ExternalDelete{},
				err: nil,
			},
			service: func() *fake.MockOrganization {
				return &fake.MockOrganization{}
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
			del, err := c.Delete(context.Background(), tc.args.mg)

			if tc.want.err != nil && err != nil {
				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("Delete(...): want error string != got error string:\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(tc.want.err, err); diff != "" {
					t.Errorf("Delete(...): want error != got error:\n%s", diff)
				}
			}
			if diff := cmp.Diff(tc.want.del, del); diff != "" {
				t.Errorf("Delete(...): -want, +got:\n%s", diff)
			}
		})
	}
}
