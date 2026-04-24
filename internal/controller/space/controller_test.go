package space

import (
	"context"
	"fmt"
	"testing"

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

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/fake"
)

var (
	errBoom     = errors.New("boom")
	name        = "my-space"
	guid        = "2d8b0d04-d537-4e4e-8c6f-f09ca0e7f56f"
	orgGuid     = "3d8b0d04-d537-4e4e-8c6f-f09ca0e7f56f"
	invalidGuid = "not-a-valid-guid"
)

type modifier func(*v1alpha1.Space)

func withExternalName(name string) modifier {
	return func(r *v1alpha1.Space) {
		r.Annotations[meta.AnnotationKeyExternalName] = name
	}
}

func withName(name string) modifier {
	return func(r *v1alpha1.Space) {
		r.Spec.ForProvider.Name = name
	}
}

func withAllowSSH(allowSSH bool) modifier {
	return func(r *v1alpha1.Space) {
		r.Spec.ForProvider.AllowSSH = allowSSH
	}
}

func withOrg(org string) modifier {
	return func(r *v1alpha1.Space) {
		r.Spec.ForProvider.Org = &org
	}
}

func withConditions(c ...xpv1.Condition) modifier {
	return func(i *v1alpha1.Space) { i.Status.SetConditions(c...) }
}

func fakeSpace(m ...modifier) *v1alpha1.Space {
	r := &v1alpha1.Space{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Finalizers:  []string{},
			Annotations: map[string]string{},
		},
		Spec: v1alpha1.SpaceSpec{
			ForProvider: v1alpha1.SpaceParameters{},
		},
	}

	for _, rm := range m {
		rm(r)
	}
	return r
}

type MockSpaceFeature struct {
	*fake.MockSpace
	*fake.MockFeature
}

func TestObserve(t *testing.T) {
	type service func() *MockSpaceFeature
	type args struct {
		mg resource.Managed
	}

	type want struct {
		mg  *v1alpha1.Space
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
				obs: managed.ExternalObservation{ResourceExists: false},
				err: errors.New(errNotSpace),
			},
			service: func() *MockSpaceFeature {
				m := &fake.MockSpace{}
				f := &fake.MockFeature{}

				return &MockSpaceFeature{m, f}
			},
		},
		// This tests whether the external API is reachable
		"Boom!": {
			args: args{
				mg: fakeSpace(withExternalName(guid)),
			},
			want: want{
				mg:  fakeSpace(withExternalName(guid)),
				obs: managed.ExternalObservation{},
				err: errors.Wrap(errBoom, errGet),
			},
			service: func() *MockSpaceFeature {
				m := &fake.MockSpace{}
				f := &fake.MockFeature{}
				m.On("Get", guid).Return(
					fake.SpaceNil,
					errBoom,
				)

				return &MockSpaceFeature{m, f}
			},
		},
		"InvalidGUID": {
			args: args{
				mg: fakeSpace(withExternalName(invalidGuid)),
			},
			want: want{
				mg:  fakeSpace(withExternalName(invalidGuid)),
				obs: managed.ExternalObservation{},
				err: errors.New(fmt.Sprintf("external-name '%s' is not a valid GUID format", invalidGuid)),
			},
			service: func() *MockSpaceFeature {
				m := &fake.MockSpace{}
				f := &fake.MockFeature{}

				return &MockSpaceFeature{m, f}
			},
		},
		"UnsetExternalNameNotFound": {
			args: args{
				mg: fakeSpace(),
			},
			want: want{
				mg: fakeSpace(),
				obs: managed.ExternalObservation{
					ResourceExists: false,
				},
				err: nil,
			},
			service: func() *MockSpaceFeature {
				m := &fake.MockSpace{}
				f := &fake.MockFeature{}

				m.On("Single").Return(
					fake.SpaceNil,
					fake.ErrNoResultReturned,
				)

				return &MockSpaceFeature{m, f}
			},
		},
		"UnsetExternalNameSuccessful": {
			args: args{
				mg: fakeSpace(withName(name), withOrg(orgGuid)),
			},
			want: want{
				mg:  fakeSpace(withName(name), withOrg(orgGuid), withExternalName(guid), withAllowSSH(false), withConditions(xpv1.Available())),
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true, ResourceLateInitialized: true},
				err: nil,
			},
			service: func() *MockSpaceFeature {
				m := &fake.MockSpace{}
				f := &fake.MockFeature{}

				m.On("Single").Return(
					&fake.NewSpace().SetName(name).SetGUID(guid).SetRelationships(orgGuid).Space,
					nil,
				)

				m.On("Get", guid).Return(
					&fake.NewSpace().SetName(name).SetGUID(guid).SetRelationships(orgGuid).Space,
					nil,
				)

				f.On("IsSSHEnabled").Return(
					false,
					nil,
				)

				return &MockSpaceFeature{m, f}
			},
		},
		"SetExternalNameNotFound": {
			args: args{
				mg: fakeSpace(withExternalName(guid)),
			},
			want: want{
				mg: fakeSpace(withExternalName(guid)),
				obs: managed.ExternalObservation{
					ResourceExists: false,
				},
				err: nil,
			},
			service: func() *MockSpaceFeature {
				m := &fake.MockSpace{}
				f := &fake.MockFeature{}

				m.On("Get", guid).Return(
					fake.SpaceNil,
					fake.ErrNoResultReturned,
				)

				return &MockSpaceFeature{m, f}
			},
		},
		"SetExternalNameSuccessful": {
			args: args{
				mg: fakeSpace(withName(name), withOrg(orgGuid), withExternalName(guid)),
			},
			want: want{
				mg:  fakeSpace(withName(name), withOrg(orgGuid), withAllowSSH(false), withExternalName(guid), withConditions(xpv1.Available())),
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true, ResourceLateInitialized: false},
				err: nil,
			},
			service: func() *MockSpaceFeature {
				m := &fake.MockSpace{}
				f := &fake.MockFeature{}

				m.On("Get", guid).Return(
					&fake.NewSpace().SetName(name).SetGUID(guid).SetRelationships(orgGuid).Space,
					nil,
				)

				f.On("IsSSHEnabled").Return(
					false,
					nil,
				)
				return &MockSpaceFeature{m, f}
			},
		},
		"Should adopt and update external-name": {
			args: args{
				mg: fakeSpace(withExternalName(guid), withName("existing-space"), withOrg(orgGuid)),
			},
			want: want{
				mg:  fakeSpace(withName("existing-space"), withExternalName(guid), withAllowSSH(false), withOrg(orgGuid), withConditions(xpv1.Available())),
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true, ResourceLateInitialized: false},
				err: nil,
			},
			service: func() *MockSpaceFeature {
				m := &fake.MockSpace{}
				f := &fake.MockFeature{}

				m.On("Single").Return(
					&fake.NewSpace().SetName("existing-space").SetGUID(guid).SetRelationships(orgGuid).Space,
					nil,
				)

				m.On("Get", guid).Return(
					&fake.NewSpace().SetName("existing-space").SetGUID(guid).SetRelationships(orgGuid).Space,
					nil,
				)

				f.On("IsSSHEnabled").Return(
					false,
					nil,
				)

				return &MockSpaceFeature{m, f}
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
				feature: tc.service().MockFeature,
				client:  tc.service().MockSpace,
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
			if tc.args.mg != nil && tc.want.mg != nil {
				if diff := cmp.Diff(tc.want.mg, tc.args.mg, cmp.Options{cmpopts.IgnoreFields(v1alpha1.Space{}, "Status.AtProvider")}); diff != "" {
					t.Errorf("Observe(...): -want, +got:\n%s", diff)
				}
			}
		})
	}
}

func TestCreate(t *testing.T) {
	type service func() *MockSpaceFeature
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
		"SuccessfulWithExternalName": {
			args: args{
				mg: fakeSpace(withExternalName(guid)),
			},
			want: want{
				mg:  fakeSpace(withExternalName(guid), withConditions(xpv1.Creating())),
				obs: managed.ExternalCreation{ConnectionDetails: managed.ConnectionDetails{}},
				err: nil,
			},
			service: func() *MockSpaceFeature {
				m := &fake.MockSpace{}
				f := &fake.MockFeature{}
				m.On("Create").Return(
					&fake.NewSpace().SetName(name).SetGUID(guid).Space,
					nil,
				)
				f.On("EnableSSH").Return(
					nil,
				)
				return &MockSpaceFeature{m, f}
			},
		},
		"SuccessfulWithoutExternalName": {
			args: args{
				mg: fakeSpace(),
			},
			want: want{
				mg:  fakeSpace(withExternalName(guid), withConditions(xpv1.Creating())),
				obs: managed.ExternalCreation{ConnectionDetails: managed.ConnectionDetails{}},
				err: nil,
			},
			service: func() *MockSpaceFeature {
				m := &fake.MockSpace{}
				f := &fake.MockFeature{}
				m.On("Create").Return(
					&fake.NewSpace().SetName(name).SetGUID(guid).Space,
					nil,
				)
				f.On("EnableSSH").Return(
					nil,
				)
				return &MockSpaceFeature{m, f}
			},
		},
		"AlreadyExistWithExternalName": {
			args: args{
				mg: fakeSpace(withExternalName(guid)),
			},
			want: want{
				mg:  fakeSpace(withExternalName(guid), withConditions(xpv1.Creating())),
				obs: managed.ExternalCreation{},
				err: errors.Wrap(errBoom, errCreate),
			},
			service: func() *MockSpaceFeature {
				m := &fake.MockSpace{}
				f := &fake.MockFeature{}

				m.On("Create").Return(
					&fake.NewSpace().SetName(name).SetGUID(guid).Space,
					errBoom,
				)
				f.On("EnableSSH").Return(
					nil,
				)
				return &MockSpaceFeature{m, f}
			},
		},
		"AlreadyExistWithoutExternalName": {
			args: args{
				mg: fakeSpace(),
			},
			want: want{
				mg:  fakeSpace(withConditions(xpv1.Creating())),
				obs: managed.ExternalCreation{},
				err: errors.Wrap(errBoom, errCreate),
			},
			service: func() *MockSpaceFeature {
				m := &fake.MockSpace{}
				f := &fake.MockFeature{}

				m.On("Create").Return(
					&fake.NewSpace().SetName(name).SetGUID(guid).Space,
					errBoom,
				)
				f.On("EnableSSH").Return(
					nil,
				)
				return &MockSpaceFeature{m, f}
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
				feature: tc.service().MockFeature,
				client:  tc.service().MockSpace,
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
			if tc.args.mg != nil && tc.want.mg != nil {
				if diff := cmp.Diff(tc.want.mg, tc.args.mg, cmp.Options{cmpopts.IgnoreFields(v1alpha1.Space{}, "Status.AtProvider")}); diff != "" {
					t.Errorf("Create(...): -want, +got:\n%s", diff)
				}
			}
		})
	}
}

func TestUpdate(t *testing.T) {
	type service func() *MockSpaceFeature
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
		"SuccessfulRename": {
			args: args{
				mg: fakeSpace(withExternalName(guid), withName(name)),
			},
			want: want{
				mg:  fakeSpace(withExternalName(guid), withName(name)),
				obs: managed.ExternalUpdate{},
				err: nil,
			},
			service: func() *MockSpaceFeature {
				m := &fake.MockSpace{}
				f := &fake.MockFeature{}
				m.On("Update").Return(
					&fake.NewSpace().SetName(name).SetGUID(guid).Space,
					nil,
				)
				return &MockSpaceFeature{m, f}
			},
		},
		"EnableSSH": {
			args: args{
				mg: fakeSpace(withExternalName(guid), withName(name), withAllowSSH(true)),
			},
			want: want{
				mg:  fakeSpace(withExternalName(guid), withName(name), withAllowSSH(true)),
				obs: managed.ExternalUpdate{},
				err: nil,
			},
			service: func() *MockSpaceFeature {
				m := &fake.MockSpace{}
				f := &fake.MockFeature{}
				f.On("EnableSSH").Return(
					nil,
				)
				m.On("Update").Return(
					&fake.NewSpace().SetName(name).SetGUID(guid).Space,
					nil,
				)
				return &MockSpaceFeature{m, f}
			},
		},
		"InvalidGUID": {
			args: args{
				mg: fakeSpace(withExternalName(invalidGuid)),
			},
			want: want{
				mg:  fakeSpace(withExternalName(invalidGuid)),
				obs: managed.ExternalUpdate{},
				err: errors.Wrap(errors.New(errUpdate), fmt.Sprintf("external-name '%s' is not a valid GUID format", invalidGuid)),
			},
			service: func() *MockSpaceFeature {
				m := &fake.MockSpace{}
				f := &fake.MockFeature{}

				return &MockSpaceFeature{m, f}
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
				feature: tc.service().MockFeature,
				client:  tc.service().MockSpace,
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
			if tc.args.mg != nil && tc.want.mg != nil {
				if diff := cmp.Diff(tc.want.mg, tc.args.mg); diff != "" {
					t.Errorf("Update(...): -want, +got:\n%s", diff)
				}
			}
		})
	}
}

func TestDelete(t *testing.T) {
	type service func() *MockSpaceFeature
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
				mg: fakeSpace(withExternalName(guid)),
			},
			want: want{
				mg:  fakeSpace(withExternalName(guid), withConditions(xpv1.Deleting())),
				obs: managed.ExternalDelete{},
				err: nil,
			},
			service: func() *MockSpaceFeature {
				m := &fake.MockSpace{}
				f := &fake.MockFeature{}
				m.On("Delete").Return(
					"",
					nil,
				)
				return &MockSpaceFeature{m, f}
			},
		},
		"404NotFound": {
			args: args{
				mg: fakeSpace(withExternalName(guid)),
			},
			want: want{
				mg:  fakeSpace(withExternalName(guid), withConditions(xpv1.Deleting())),
				obs: managed.ExternalDelete{},
				err: nil,
			},
			service: func() *MockSpaceFeature {
				m := &fake.MockSpace{}
				f := &fake.MockFeature{}
				m.On("Delete").Return(
					"",
					fake.ErrNoResultReturned,
				)
				return &MockSpaceFeature{m, f}
			},
		},
		"InvalidGUID": {
			args: args{
				mg: fakeSpace(withExternalName(invalidGuid)),
			},
			want: want{
				mg:  fakeSpace(withExternalName(invalidGuid), withConditions(xpv1.Deleting())),
				obs: managed.ExternalDelete{},
				err: errors.Wrap(errors.New(errDelete), fmt.Sprintf("external-name '%s' is not a valid GUID format", invalidGuid)),
			},
			service: func() *MockSpaceFeature {
				m := &fake.MockSpace{}
				f := &fake.MockFeature{}
				return &MockSpaceFeature{m, f}
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
				feature: tc.service().MockFeature,
				client:  tc.service().MockSpace,
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

func TestInitialize(t *testing.T) {
	type service func() *MockSpaceFeature
	type args struct {
		mg resource.Managed
	}

	type want struct {
		mg  *v1alpha1.Space
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
				err: errors.New(errNotSpace),
			},
			service: func() *MockSpaceFeature {
				m := &fake.MockSpace{}
				f := &fake.MockFeature{}

				return &MockSpaceFeature{m, f}
			},
		},
		"SuccessfulWithOrgGUID": {
			args: args{
				mg: fakeSpace(
					withExternalName(guid), withName(name), withOrg(orgGuid),
				),
			},
			want: want{
				mg: fakeSpace(
					withExternalName(guid),
					withName(name),
					withAllowSSH(false),
					withOrg(orgGuid),
				),
				err: nil,
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			c := &orgInitializer{
				kube: &test.MockClient{
					MockUpdate:       test.NewMockUpdateFn(nil),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
			}
			err := c.Initialize(context.Background(), tc.args.mg)

			if tc.want.err != nil && err != nil {
				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("Initialize(...): want error string != got error string:\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(tc.want.err, err); diff != "" {
					t.Errorf("Initialize(...): want error != got error:\n%s", diff)
				}
			}
		})
	}
}
