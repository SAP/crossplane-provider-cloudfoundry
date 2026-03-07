package org

import (
	"context"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
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
		r.ObjectMeta.Annotations[meta.AnnotationKeyExternalName] = name
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
		"Error if cr is not the right kind": {
			args: args{
				mg: nil,
			},
			want: want{
				mg:  nil,
				obs: managed.ExternalObservation{ResourceExists: false},
				err: errors.New(errNotOrgKind),
			},
			service: func() *fake.MockOrganization {
				m := &fake.MockOrganization{}
				return m
			},
		},
		// This tests whether the external API is reachable
		"Error when external API is not working": {
			args: args{
				mg: fakeOrg(withExternalName(guid)),
			},
			want: want{
				mg:  fakeOrg(withExternalName(guid)),
				obs: managed.ExternalObservation{},
				err: errors.Wrap(errBoom, errGetResource),
			},
			service: func() *fake.MockOrganization {
				m := &fake.MockOrganization{}
				m.On("Get", guid).Return(
					fake.OrganizationNil,
					errBoom,
				)
				m.On("Single").Return(
					fake.OrganizationNil,
					errBoom,
				)
				return m
			},
		},
		"NotFound if org with guid is not found. Match by name should not be called": {
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
				m.On("Single").Return( // this should not be called
					&fake.NewOrganization().SetName(name).SetGUID(guid).Organization,
					nil,
				)
				return m
			},
			kube: &test.MockClient{},
		},
		"NotFound by uuid is not provided and org with name is not found": {
			args: args{
				mg: fakeOrg(withName(name), withExternalName("not-a-uuid")),
			},
			want: want{
				mg: fakeOrg(withName(name), withExternalName(guid)),
				obs: managed.ExternalObservation{
					ResourceExists: false,
				},
				err: nil,
			},
			service: func() *fake.MockOrganization {
				m := &fake.MockOrganization{}
				m.On("Get", "").Return( // this should be called
					fake.OrganizationNil,
					errBoom,
				)
				m.On("Single").Return(
					fake.OrganizationNil,
					fake.ErrNoResultReturned,
				)
				return m
			},
		},
		"Successful when org with guid is found": {
			args: args{
				mg: fakeOrg(
					withExternalName(guid),
					withName(name),
				),
			},
			want: want{
				mg: fakeOrg(
					withExternalName(guid),
					withName(name),
				),
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
				err: nil,
			},
			service: func() *fake.MockOrganization {
				m := &fake.MockOrganization{}

				m.On("Get", guid).Return(
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
		"Successful when org with guid is found, even after rename": {
			args: args{
				mg: fakeOrg(
					withExternalName(guid),
					withName("not-my-org"),
				),
			},
			want: want{
				mg: fakeOrg(
					withExternalName(guid),
					withName("not-my-org"),
				),
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: false},
				err: nil,
			},
			service: func() *fake.MockOrganization {
				m := &fake.MockOrganization{}

				m.On("Get", guid).Return(
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
		"Successful when guid is not provided and org with name is found ": {
			args: args{
				mg: fakeOrg(withName(name)),
			},
			want: want{
				mg: fakeOrg(withName(name), withExternalName(guid)),
				obs: managed.ExternalObservation{
					ResourceExists:   true,
					ResourceUpToDate: true,
				},
				err: nil,
			},
			service: func() *fake.MockOrganization {
				m := &fake.MockOrganization{}
				m.On("Get", "").Return(
					fake.OrganizationNil,
					fake.ErrNoResultReturned,
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
