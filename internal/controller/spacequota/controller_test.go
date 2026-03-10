package spacequota

import (
	"context"
	"testing"
	"time"

	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/fake"
)

var (
	errBoom = errors.New("boom")
	name    = "my-space-quota"
	guid    = "2d8b0d04-d537-4e4e-8c6f-f09ca0e7f56a"
)

type modifier func(*v1alpha1.SpaceQuota)

func withExternalName(name string) modifier {
	return func(r *v1alpha1.SpaceQuota) {
		r.ObjectMeta.Annotations[meta.AnnotationKeyExternalName] = name
	}
}

func withName(name string) modifier {
	return func(r *v1alpha1.SpaceQuota) {
		r.Spec.ForProvider.Name = &name
		r.Status.AtProvider.Name = &name
	}
}

func withOrg(org string) modifier {
	return func(r *v1alpha1.SpaceQuota) {
		r.Spec.ForProvider.Org = &org
	}
}

func withID(guid string) modifier {
	return func(r *v1alpha1.SpaceQuota) {
		r.Status.AtProvider.ID = &guid
	}
}

var zeroTime = time.Time{}.Format(time.RFC3339)

func fakeSpaceQuota(m ...modifier) *v1alpha1.SpaceQuota {
	r := &v1alpha1.SpaceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Finalizers:  []string{},
			Annotations: map[string]string{},
		},
		Spec: v1alpha1.SpaceQuotaSpec{
			ForProvider: v1alpha1.SpaceQuotaParameters{
				Spaces: []*string{},
			},
		},
		Status: v1alpha1.SpaceQuotaStatus{
			AtProvider: v1alpha1.SpaceQuotaObservation{
				CreatedAt:             &zeroTime,
				UpdatedAt:             &zeroTime,
				AllowPaidServicePlans: ptr.To(false),
			},
		},
	}
	for _, rm := range m {
		rm(r)
	}

	return r
}

func TestObserve(t *testing.T) {
	type args struct {
		mg resource.Managed
	}
	type want struct {
		mg  *v1alpha1.SpaceQuota
		obs managed.ExternalObservation
		err error
	}
	cases := map[string]struct {
		args     args
		want     want
		cfClient *fake.MockSpaceQuota
	}{
		"Nil": {
			args: args{
				mg: nil,
			},
			want: want{
				mg:  nil,
				obs: managed.ExternalObservation{ResourceExists: false},
				err: errors.New(errUnexpectedObject),
			},
			cfClient: &fake.MockSpaceQuota{},
		},
		"ExternalNameNotSet": {
			args: args{
				mg: fakeSpaceQuota(),
			},
			want: want{
				mg: fakeSpaceQuota(),
				obs: managed.ExternalObservation{
					ResourceExists: false,
				},
				err: nil,
			},
			cfClient: &fake.MockSpaceQuota{},
		},
		// This tests whether the external API is reachable
		"Boom!": {
			args: args{
				mg: fakeSpaceQuota(
					withExternalName(guid),
				),
			},
			want: want{
				mg: fakeSpaceQuota(
					withExternalName(guid),
				),
				obs: managed.ExternalObservation{},
				err: errors.Wrap(errBoom, errGet),
			},
			cfClient: func() *fake.MockSpaceQuota {
				m := &fake.MockSpaceQuota{}
				m.On("Get", guid).Return(
					fake.SpaceQuotaNil,
					errBoom,
				)
				return m
			}(),
		},
		"NotFound": {
			args: args{
				mg: fakeSpaceQuota(
					withExternalName(guid),
				),
			},
			want: want{
				mg: fakeSpaceQuota(
					withExternalName(guid),
				),
				obs: managed.ExternalObservation{ResourceExists: false},
				err: nil,
			},
			cfClient: func() *fake.MockSpaceQuota {
				m := &fake.MockSpaceQuota{}
				m.On("Get", guid).Return(
					fake.SpaceQuotaNil,
					fake.ErrNoResultReturned,
				)
				return m
			}(),
		},
		"Successful": {
			args: args{
				mg: fakeSpaceQuota(
					withExternalName(guid),
					withName(name),
					withOrg(guid),
				),
			},
			want: want{
				mg: fakeSpaceQuota(
					withExternalName(guid),
					withName(name),
					withOrg(guid),
				),
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
				err: nil,
			},
			cfClient: func() *fake.MockSpaceQuota {
				m := &fake.MockSpaceQuota{}

				m.On("Get", guid).Return(
					&fake.NewSpaceQuota().SetName(name).SetGUID(guid).SetOrgGUID(guid).SpaceQuota,
					nil,
				)

				return m
			}(),
		},
	}

	for slogan, tc := range cases {
		t.Log(slogan)
		c := &external{
			kube:   &test.MockClient{},
			client: tc.cfClient,
			isUpToDate: func(context.Context, *v1alpha1.SpaceQuota, *cfresource.SpaceQuota) (bool, error) {
				return true, nil
			},
		}

		obs, err := c.Observe(context.Background(), tc.args.mg)

		var spaceQuota *v1alpha1.SpaceQuota
		if tc.args.mg != nil {
			spaceQuota, _ = tc.args.mg.(*v1alpha1.SpaceQuota)
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
		if spaceQuota != nil && tc.want.mg != nil {
			if diff := cmp.Diff(spaceQuota.Spec, tc.want.mg.Spec); diff != "" {
				t.Errorf("Observe(...): -want, +got:\n%s", diff)
			}
		}
	}
}

func TestCreate(t *testing.T) {
	type args struct {
		mg resource.Managed
	}

	type want struct {
		mg  resource.Managed
		obs managed.ExternalCreation
		err error
	}

	cases := map[string]struct {
		args     args
		want     want
		cfClient *fake.MockSpaceQuota
	}{
		"Successful": {
			args: args{
				mg: fakeSpaceQuota(withExternalName(guid)),
			},
			want: want{
				mg:  fakeSpaceQuota(withExternalName(guid)),
				obs: managed.ExternalCreation{},
				err: nil,
			},
			cfClient: func() *fake.MockSpaceQuota {
				m := &fake.MockSpaceQuota{}
				m.On("Create").Return(
					&fake.NewSpaceQuota().SetName(name).SetGUID(guid).SpaceQuota,
					nil,
				)
				return m
			}(),
		},
		"AlreadyExist": {
			args: args{
				mg: fakeSpaceQuota(withExternalName(guid)),
			},
			want: want{
				mg:  fakeSpaceQuota(withExternalName(guid)),
				obs: managed.ExternalCreation{},
				err: errors.Wrap(errBoom, errCreate),
			},
			cfClient: func() *fake.MockSpaceQuota {
				m := &fake.MockSpaceQuota{}

				m.On("Create").Return(
					&fake.NewSpaceQuota().SetName(name).SetGUID(guid).SpaceQuota,
					errBoom,
				)
				return m
			}(),
		},
	}

	for slogan, tc := range cases {
		t.Log(slogan)
		c := &external{
			kube:   &test.MockClient{},
			client: tc.cfClient,
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
	}
}

func TestUpdate(t *testing.T) {
	type args struct {
		mg resource.Managed
	}

	type want struct {
		mg  resource.Managed
		obs managed.ExternalUpdate
		err error
	}

	cases := map[string]struct {
		args     args
		want     want
		cfClient *fake.MockSpaceQuota
	}{
		"SuccessfulRename": {
			args: args{
				mg: fakeSpaceQuota(withExternalName(guid), withID(guid), withName(name)),
			},
			want: want{
				mg:  fakeSpaceQuota(withExternalName(guid), withID(guid), withName(name)),
				obs: managed.ExternalUpdate{},
				err: nil,
			},
			cfClient: func() *fake.MockSpaceQuota {
				m := &fake.MockSpaceQuota{}
				m.On("Update").Return(
					&fake.NewSpaceQuota().SetName(name).SetGUID(guid).SpaceQuota,
					nil,
				)
				return m
			}(),
		},
		"IDNotSet": {
			args: args{
				mg: fakeSpaceQuota(withExternalName(guid)),
			},
			want: want{
				mg:  fakeSpaceQuota(withExternalName(guid)),
				obs: managed.ExternalUpdate{},
				err: errors.New(errUpdate),
			},
			cfClient: &fake.MockSpaceQuota{},
		},
	}

	for slogan, tc := range cases {
		t.Log(slogan)
		c := &external{
			kube:   &test.MockClient{},
			client: tc.cfClient,
		}

		obs, err := c.Update(context.Background(), tc.args.mg)

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
	}
}

func TestDelete(t *testing.T) {
	type args struct {
		mg resource.Managed
	}

	type want struct {
		mg  resource.Managed
		err error
	}

	cases := map[string]struct {
		args     args
		want     want
		cfClient *fake.MockSpaceQuota
	}{
		"SuccessfulDelete": {
			args: args{
				mg: fakeSpaceQuota(withExternalName(guid), withID(guid)),
			},
			want: want{
				mg:  fakeSpaceQuota(withExternalName(guid), withID(guid)),
				err: nil,
			},
			cfClient: func() *fake.MockSpaceQuota {
				m := &fake.MockSpaceQuota{}
				m.On("Delete").Return(
					"",
					nil,
				)
				return m
			}(),
		},
		"IDNotSet": {
			args: args{
				mg: fakeSpaceQuota(withExternalName(guid)),
			},
			want: want{
				mg:  fakeSpaceQuota(withExternalName(guid)),
				err: errors.New(errDelete),
			},
			cfClient: &fake.MockSpaceQuota{},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			t.Logf("Testing: %s", t.Name())
			c := &external{
				kube:   &test.MockClient{},
				client: tc.cfClient,
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
