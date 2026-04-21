package spacequota

import (
	"context"
	"fmt"
	"testing"
	"time"

	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/fake"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

var (
	errBoom       = errors.New("boom")
	name          = "my-space-quota"
	guid          = "2d8b0d04-d537-4e4e-8c6f-f09ca0e7f56a"
	differentGuid = "311a35b7-a28c-402f-b221-ba8c16de32cc"
	invalidGuid   = "invalid-guid"
)

type modifier func(*v1alpha1.SpaceQuota)

func withExternalName(name string) modifier {
	return func(r *v1alpha1.SpaceQuota) {
		r.Annotations[meta.AnnotationKeyExternalName] = name
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

func withSpace(guid string) modifier {
	return func(r *v1alpha1.SpaceQuota) {
		r.Spec.ForProvider.Spaces = []*string{&guid}
	}
}

func withConditions(c ...xpv1.Condition) modifier {
	return func(i *v1alpha1.SpaceQuota) { i.Status.SetConditions(c...) }
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
		mg            resource.Managed
		checkUptoDate bool
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
		"InvalidGuid": {
			args: args{
				mg: fakeSpaceQuota(
					withExternalName(invalidGuid),
				),
			},
			want: want{
				mg: fakeSpaceQuota(
					withExternalName(invalidGuid),
				),
				obs: managed.ExternalObservation{ResourceExists: false},
				err: errors.New(fmt.Sprintf("external-name '%s' is not a valid GUID format", invalidGuid)),
			},
			cfClient: func() *fake.MockSpaceQuota {
				m := &fake.MockSpaceQuota{}
				return m
			}(),
		},
		"OutOfDate_OrgChanged": {
			args: args{
				mg: fakeSpaceQuota(
					withExternalName(guid),
					withName(name),
					withOrg(differentGuid),
					withSpace(guid),
				),
				checkUptoDate: true,
			},
			want: want{
				mg: fakeSpaceQuota(
					withExternalName(guid),
					withName(name),
					withOrg(differentGuid),
					withSpace(guid),
					withConditions(xpv1.Available()),
				),
				obs: managed.ExternalObservation{ResourceExists: false, ResourceUpToDate: false},
				err: errors.Wrap(errors.New(errUpdateOrg), "isUpToDate check failed"),
			},
			cfClient: func() *fake.MockSpaceQuota {
				m := &fake.MockSpaceQuota{}

				m.On("Get", guid).Return(
					&fake.NewSpaceQuota().SetName(name).SetGUID(guid).SetOrgGUID(guid).SetSpaces([]*string{&guid}).SpaceQuota,
					nil,
				)

				return m
			}(),
		},
		"OutOfDate_SpaceChanged": {
			args: args{
				mg: fakeSpaceQuota(
					withExternalName(guid),
					withName(name),
					withOrg(guid),
					withSpace(differentGuid),
				),
				checkUptoDate: true,
			},
			want: want{
				mg: fakeSpaceQuota(
					withExternalName(guid),
					withName(name),
					withOrg(guid),
					withSpace(differentGuid),
					withConditions(xpv1.Available()),
				),
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: false},
				err: nil,
			},
			cfClient: func() *fake.MockSpaceQuota {
				m := &fake.MockSpaceQuota{}

				m.On("Get", guid).Return(
					&fake.NewSpaceQuota().SetName(name).SetGUID(guid).SetOrgGUID(guid).SetSpaces([]*string{&guid}).SpaceQuota,
					nil,
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
					withConditions(xpv1.Available()),
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

		if tc.args.checkUptoDate {
			c.isUpToDate = isUpToDate
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
		if tc.want.mg != nil && tc.args.mg != nil {
			if diff := cmp.Diff(tc.want.mg, tc.args.mg, cmp.Options{cmpopts.IgnoreFields(v1alpha1.SpaceQuota{}, "Status.AtProvider")}); diff != "" {
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
		mg  *v1alpha1.SpaceQuota
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
				mg:  fakeSpaceQuota(withExternalName(guid), withConditions(xpv1.Creating())),
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
				mg:  fakeSpaceQuota(withExternalName(guid), withConditions(xpv1.Creating())),
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
		"AlreadyExistWithoutExternalName": {
			args: args{
				mg: fakeSpaceQuota(withName(name)),
			},
			want: want{
				mg:  fakeSpaceQuota(withName(name), withConditions(xpv1.Creating())),
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
		if tc.want.mg != nil && tc.args.mg != nil {
			if diff := cmp.Diff(tc.want.mg, tc.args.mg, cmp.Options{cmpopts.IgnoreFields(v1alpha1.SpaceQuota{}, "Status.AtProvider")}); diff != "" {
				t.Errorf("Create(...): -want, +got:\n%s", diff)
			}
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
				err: errors.Wrap(errBoom, errUpdate),
			},
			cfClient: func() *fake.MockSpaceQuota {
				m := &fake.MockSpaceQuota{}
				m.On("Update").Return(
					&fake.NewSpaceQuota().SpaceQuota,
					errBoom,
				)
				return m
			}(),
		},
		"OutOfDate_OrgChanged": {
			args: args{
				mg: fakeSpaceQuota(
					withExternalName(guid),
					withName(name),
					withOrg(guid),
					withSpace(guid),
				),
			},
			want: want{
				mg: fakeSpaceQuota(
					withExternalName(guid),
					withName(name),
					withOrg(guid),
					withSpace(guid),
				),
				obs: managed.ExternalUpdate{},
				err: errors.Wrap(errBoom, errUpdate),
			},
			cfClient: func() *fake.MockSpaceQuota {
				m := &fake.MockSpaceQuota{}

				m.On("Update").Return(
					&fake.NewSpaceQuota().SetName(name).SetGUID(guid).SetOrgGUID(guid).SetSpaces([]*string{&guid}).SpaceQuota,
					errBoom,
				)

				return m
			}(),
		},
		"ExternalNameNotSet": {
			args: args{
				mg: fakeSpaceQuota(withName(name)),
			},
			want: want{
				mg:  fakeSpaceQuota(withName(name)),
				obs: managed.ExternalUpdate{},
				err: errors.New(errUpdate + ": " + errExternalName),
			},
			cfClient: func() *fake.MockSpaceQuota {
				m := &fake.MockSpaceQuota{}
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
		if tc.args.mg != nil && tc.want.mg != nil {
			if diff := cmp.Diff(tc.want.mg, tc.args.mg); diff != "" {
				t.Errorf("Observe(...): -want, +got:\n%s", diff)
			}
		}

	}
}

func TestDelete(t *testing.T) {
	type args struct {
		mg resource.Managed
	}

	type want struct {
		mg  resource.Managed
		obs managed.ExternalDelete
		err error
	}

	cases := map[string]struct {
		args     args
		want     want
		cfClient *fake.MockSpaceQuota
	}{
		"SuccessfulDelete": {
			args: args{
				mg: fakeSpaceQuota(withExternalName(guid)),
			},
			want: want{
				mg:  fakeSpaceQuota(withExternalName(guid), withConditions(xpv1.Deleting())),
				obs: managed.ExternalDelete{},
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
		"404NotFound": {
			args: args{
				mg: fakeSpaceQuota(withExternalName(guid)),
			},
			want: want{
				mg:  fakeSpaceQuota(withExternalName(guid), withConditions(xpv1.Deleting())),
				obs: managed.ExternalDelete{},
				err: nil,
			},
			cfClient: func() *fake.MockSpaceQuota {
				m := &fake.MockSpaceQuota{}
				m.On("Delete").Return(
					"",
					errors.New("CF-ResourceNotFound: The space quota was not found"),
				)
				return m
			}(),
		},
		"ExternalNameNotSet": {
			args: args{
				mg: fakeSpaceQuota(),
			},
			want: want{
				mg:  fakeSpaceQuota(withConditions(xpv1.Deleting())),
				obs: managed.ExternalDelete{},
				err: errors.New(errDelete + ": " + errExternalName),
			},
			cfClient: func() *fake.MockSpaceQuota {
				m := &fake.MockSpaceQuota{}
				return m
			}(),
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
					t.Errorf("Delete(...): want error string != got error string:\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(tc.want.err, err); diff != "" {
					t.Errorf("Delete(...): want error != got error:\n%s", diff)
				}
			}
			if tc.args.mg != nil && tc.want.mg != nil {
				if diff := cmp.Diff(tc.want.mg, tc.args.mg); diff != "" {
					t.Errorf("Delete(...): -want, +got:\n%s", diff)
				}
			}
		})
	}
}
