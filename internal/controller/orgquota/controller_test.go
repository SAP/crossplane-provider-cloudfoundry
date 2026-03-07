package orgquota

import (
	"context"
	"testing"

	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
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
	guid        = "33fd5b0b-4f3b-4b1b-8b3d-3b5f7b4b3b4b"
	name        = "test-org-quota"
	errBoom     = errors.New("boom")
	nilOrgQuota *cfresource.OrganizationQuota
)

type modifier func(*v1alpha1.OrgQuota)

func withExternalName(guid string) modifier {
	return func(r *v1alpha1.OrgQuota) {
		r.ObjectMeta.Annotations[meta.AnnotationKeyExternalName] = guid
	}
}

func withName(name string) modifier {
	return func(r *v1alpha1.OrgQuota) {
		r.Spec.ForProvider.Name = &name
	}
}

func withAllowPaidServicePlans(allow bool) modifier {
	return func(r *v1alpha1.OrgQuota) {
		r.Spec.ForProvider.AllowPaidServicePlans = &allow
	}
}

func withConditions(c ...xpv1.Condition) modifier {
	return func(r *v1alpha1.OrgQuota) { r.Status.SetConditions(c...) }
}

func withNilID() modifier {
	return func(r *v1alpha1.OrgQuota) {
		r.Status.AtProvider.ID = nil
	}
}

func fakeOrgQuota(m ...modifier) *v1alpha1.OrgQuota {
	r := &v1alpha1.OrgQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Finalizers:  []string{},
			Annotations: map[string]string{},
		},
		Spec: v1alpha1.OrgQuotaSpec{
			ForProvider: v1alpha1.OrgQuotaParameters{
				Name: ptr.To("test-org-quota"),
			},
		},
		Status: v1alpha1.OrgQuotaStatus{
			AtProvider: v1alpha1.OrgQuotaObservation{
				ID: ptr.To(guid),
			},
		},
	}

	for _, rm := range m {
		rm(r)
	}
	return r
}

func fakeOrgQuotaResource(id string, p bool) *cfresource.OrganizationQuota {
	r := &cfresource.OrganizationQuota{}
	r.GUID = id
	r.Name = "test-org-quota"
	r.Services.PaidServicesAllowed = p
	return r
}

func TestObserve(t *testing.T) {
	type service func() *fake.MockOrgQuota
	type args struct {
		mg resource.Managed
	}

	type want struct {
		mg  *v1alpha1.OrgQuota
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
				obs: managed.ExternalObservation{ResourceExists: false},
				err: errors.New(errNotOrgQuota),
			},
			service: func() *fake.MockOrgQuota {
				m := &fake.MockOrgQuota{}
				return m
			},
		},
		// This tests whether the external API is reachable
		"Error when external API is not working": {
			args: args{
				mg: fakeOrgQuota(withExternalName(guid)),
			},
			want: want{
				mg:  fakeOrgQuota(withExternalName(guid)),
				obs: managed.ExternalObservation{},
				err: errors.Wrap(errBoom, errGet),
			},
			service: func() *fake.MockOrgQuota {
				m := &fake.MockOrgQuota{}
				m.On("Get", guid).Return(
					nilOrgQuota,
					errBoom,
				)
				return m
			},
		},
		"NotFound when external name is empty": {
			args: args{
				mg: fakeOrgQuota(),
			},
			want: want{
				mg: fakeOrgQuota(withExternalName(name)),
				obs: managed.ExternalObservation{
					ResourceExists: false,
				},
				err: errors.Wrap(errors.New("not found"), errGet),
			},
			service: func() *fake.MockOrgQuota {
				m := &fake.MockOrgQuota{}

				m.On("Get", name).Return(
					nilOrgQuota,
					errors.New("not found"),
				)
				return m
			},
		},
		"NotFound when Get returns not found error": {
			args: args{
				mg: fakeOrgQuota(withExternalName(guid)),
			},
			want: want{
				mg: fakeOrgQuota(withExternalName(guid)),
				obs: managed.ExternalObservation{
					ResourceExists: false,
				},
				err: errors.Wrap(errors.New("not found"), errGet),
			},
			service: func() *fake.MockOrgQuota {
				m := &fake.MockOrgQuota{}
				m.On("Get", guid).Return(
					nilOrgQuota,
					errors.New("not found"),
				)
				return m
			},
		},
		"Found with observation is returned": {
			args: args{
				mg: fakeOrgQuota(
					withExternalName(guid),
					withName("test-quota"),
					withAllowPaidServicePlans(true),
				),
			},
			want: want{
				mg: fakeOrgQuota(
					withExternalName(guid),
					withName("test-quota"),
					withAllowPaidServicePlans(true),
				),
				obs: managed.ExternalObservation{
					ResourceExists:          true,
					ResourceUpToDate:        false,
					ResourceLateInitialized: true,
				},
				err: nil,
			},
			service: func() *fake.MockOrgQuota {
				m := &fake.MockOrgQuota{}
				m.On("Get", guid).Return(
					fakeOrgQuotaResource(guid, true),
					nil,
				)
				return m
			},
		},
		"Found using metadata.name as external-name": {
			args: args{
				mg: fakeOrgQuota(),
			},
			want: want{
				mg: fakeOrgQuota(
					withExternalName("test-org-quota"),
					withName("test-quota"),
					withAllowPaidServicePlans(true),
				),
				obs: managed.ExternalObservation{
					ResourceExists:          true,
					ResourceUpToDate:        false,
					ResourceLateInitialized: true,
				},
				err: nil,
			},
			service: func() *fake.MockOrgQuota {
				m := &fake.MockOrgQuota{}
				m.On("Get", "test-org-quota").Return(
					fakeOrgQuotaResource(guid, true),
					nil,
				)
				return m
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			c := &externalClient{
				kubeClient: &test.MockClient{
					MockUpdate:       test.NewMockUpdateFn(nil),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				cloudFoundryClient: tc.service(),
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
	type service func() *fake.MockOrgQuota
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
				mg: fakeOrgQuota(
					withName("test-quota"),
				),
			},
			want: want{
				mg: fakeOrgQuota(
					withName("test-quota"),
					withExternalName(guid),
					withAllowPaidServicePlans(true),
				),
				obs: managed.ExternalCreation{},
				err: nil,
			},
			service: func() *fake.MockOrgQuota {
				m := &fake.MockOrgQuota{}
				m.On("Create").Return(
					fakeOrgQuotaResource(guid, true),
					nil,
				)
				return m
			},
		},
		"Failed": {
			args: args{
				mg: fakeOrgQuota(
					withName("test-quota"),
					withAllowPaidServicePlans(true),
				),
			},
			want: want{
				mg: fakeOrgQuota(
					withName("test-quota"),
					withAllowPaidServicePlans(true),
				),
				obs: managed.ExternalCreation{},
				err: errors.Wrap(errBoom, errCreate),
			},
			service: func() *fake.MockOrgQuota {
				m := &fake.MockOrgQuota{}
				m.On("Create").Return(
					(*cfresource.OrganizationQuota)(nil),
					errBoom,
				)
				return m
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			c := &externalClient{
				kubeClient: &test.MockClient{
					MockUpdate:       test.NewMockUpdateFn(nil),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				cloudFoundryClient: tc.service(),
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
		})
	}
}

func TestUpdate(t *testing.T) {
	type service func() *fake.MockOrgQuota
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
		"Successful": {
			args: args{
				mg: fakeOrgQuota(
					withExternalName(guid),
					withName("test-quota"),
					withAllowPaidServicePlans(true),
				),
			},
			want: want{
				mg: fakeOrgQuota(
					withExternalName(guid),
					withName("test-quota"),
					withAllowPaidServicePlans(true),
				),
				obs: managed.ExternalUpdate{},
				err: nil,
			},
			service: func() *fake.MockOrgQuota {
				m := &fake.MockOrgQuota{}
				m.On("Update").Return(
					fakeOrgQuotaResource(guid, true),
					nil,
				)
				return m
			},
		},
		"Failed": {
			args: args{
				mg: fakeOrgQuota(
					withExternalName(guid),
					withName("test-quota"),
					withAllowPaidServicePlans(true),
				),
			},
			want: want{
				mg: fakeOrgQuota(
					withExternalName(guid),
					withName("test-quota"),
					withAllowPaidServicePlans(true),
				),
				obs: managed.ExternalUpdate{},
				err: errors.Wrap(errBoom, errUpdate),
			},
			service: func() *fake.MockOrgQuota {
				m := &fake.MockOrgQuota{}
				m.On("Update").Return(
					(*cfresource.OrganizationQuota)(nil),
					errBoom,
				)
				return m
			},
		},
		"Failed because nil ID": {
			args: args{
				mg: fakeOrgQuota(
					withExternalName(guid),
					withName("test-quota"),
					withNilID(),
				),
			},
			want: want{
				mg: fakeOrgQuota(
					withExternalName(guid),
					withName("test-quota"),
					withNilID(),
				),
				err: errors.New(errUpdate),
			},
			service: func() *fake.MockOrgQuota {
				m := &fake.MockOrgQuota{}
				m.On("Update").Return(
					"",
					errBoom,
				)
				return m
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			c := &externalClient{
				kubeClient: &test.MockClient{
					MockUpdate:       test.NewMockUpdateFn(nil),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				cloudFoundryClient: tc.service(),
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
		})
	}
}

func TestDelete(t *testing.T) {
	type service func() *fake.MockOrgQuota
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
		"Successful": {
			args: args{
				mg: fakeOrgQuota(
					withExternalName(guid),
					withName("test-quota"),
				),
			},
			want: want{
				mg: fakeOrgQuota(
					withExternalName(guid),
					withName("test-quota"),
					withConditions(xpv1.Deleting()),
				),
				err: nil,
			},
			service: func() *fake.MockOrgQuota {
				m := &fake.MockOrgQuota{}
				m.On("Delete").Return(
					"",
					nil,
				)
				return m
			},
		},
		"Failed": {
			args: args{
				mg: fakeOrgQuota(
					withExternalName(guid),
					withName("test-quota"),
				),
			},
			want: want{
				mg: fakeOrgQuota(
					withExternalName(guid),
					withName("test-quota"),
					withConditions(xpv1.Deleting()),
				),
				err: errors.Wrap(errBoom, errDelete),
			},
			service: func() *fake.MockOrgQuota {
				m := &fake.MockOrgQuota{}
				m.On("Delete").Return(
					"",
					errBoom,
				)
				return m
			},
		},
		"Failed because nil ID": {
			args: args{
				mg: fakeOrgQuota(
					withExternalName(guid),
					withName("test-quota"),
					withNilID(),
				),
			},
			want: want{
				mg: fakeOrgQuota(
					withExternalName(guid),
					withName("test-quota"),
					withConditions(xpv1.Deleting()),
					withNilID(),
				),
				err: errors.Wrap(errors.New(".Status.AtProvider.ID is not set"), errDelete),
			},
			service: func() *fake.MockOrgQuota {
				m := &fake.MockOrgQuota{}
				m.On("Delete").Return(
					"",
					nil,
				)
				return m
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			c := &externalClient{
				kubeClient: &test.MockClient{
					MockUpdate:       test.NewMockUpdateFn(nil),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				cloudFoundryClient: tc.service(),
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
