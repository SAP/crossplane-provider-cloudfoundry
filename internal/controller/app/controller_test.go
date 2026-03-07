package app

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/app"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/fake"
)

var (
	errBoom   = errors.New("boom")
	name      = "my-app"
	spaceGUID = "a46808d1-d09a-4eef-add1-30872dec82f7"
	guid      = "2d8b0d04-d537-4e4e-8c6f-f09ca0e7f56f"
)

type modifier func(*v1alpha1.App)

func withExternalName(name string) modifier {
	return func(r *v1alpha1.App) {
		r.ObjectMeta.Annotations[meta.AnnotationKeyExternalName] = name
	}
}

func withSpace(space string) modifier {
	return func(r *v1alpha1.App) {
		r.Spec.ForProvider.Space = &space
	}
}

func withConditions(c ...xpv1.Condition) modifier {
	return func(i *v1alpha1.App) { i.Status.SetConditions(c...) }
}

func withStatus(guid, state string) modifier {
	o := v1alpha1.AppObservation{}
	o.GUID = guid
	o.State = state

	return func(r *v1alpha1.App) {
		r.Status.AtProvider = o
	}
}

func withImage(image string) modifier {
	return func(r *v1alpha1.App) {
		r.Spec.ForProvider.Docker = &v1alpha1.DockerConfiguration{Image: image}
	}
}

func newApp(typ string, m ...modifier) *v1alpha1.App {
	r := &v1alpha1.App{

		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Finalizers:  []string{},
			Annotations: map[string]string{},
		},
		Spec: v1alpha1.AppSpec{
			ForProvider: v1alpha1.AppParameters{Name: name, Lifecycle: typ},
		},
		Status: v1alpha1.AppStatus{
			AtProvider: v1alpha1.AppObservation{},
		},
	}

	for _, rm := range m {
		rm(r)
	}
	return r
}

func newMockPush() *fake.MockPush {
	m := &fake.MockPush{}
	m.On("GenerateManifest", guid).Return("applicationmanifest", nil)
	m.On("Push").Return(&fake.NewApp("docker").SetName(name).SetGUID(guid).App,
		nil,
	)
	return m

}

func TestObserve(t *testing.T) {
	type service func() *fake.MockApp
	type args struct {
		mg resource.Managed
	}

	type want struct {
		mg  resource.Managed
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
				obs: managed.ExternalObservation{ResourceExists: false},
				err: errors.New(errWrongKind),
			},
			service: func() *fake.MockApp {
				m := &fake.MockApp{}
				return m
			},
		},
		"ExternalNameNotSet": {
			args: args{
				mg: newApp("docker", withSpace(spaceGUID)),
			},
			want: want{
				mg: newApp("docker", withSpace(spaceGUID)),
				obs: managed.ExternalObservation{
					ResourceExists: false,
				},
				err: nil,
			},
			service: func() *fake.MockApp {
				m := &fake.MockApp{}
				m.On("Single").Return(
					fake.AppNil,
					fake.ErrNoResultReturned,
				)
				return m
			},
		},
		"Boom!": {
			args: args{
				mg: newApp("docker", withExternalName(guid), withSpace(spaceGUID)),
			},
			want: want{
				mg:  newApp("docker", withExternalName(guid)),
				obs: managed.ExternalObservation{},
				err: errors.Wrap(errBoom, errObserveResource),
			},
			service: func() *fake.MockApp {
				m := &fake.MockApp{}
				m.On("Get", guid).Return(
					fake.AppNil,
					errBoom,
				)
				m.On("Single").Return(
					fake.AppNil,
					errBoom,
				)
				return m
			},
		},
		"Should adopt": {
			args: args{
				mg: newApp("docker", withSpace(spaceGUID)),
			},
			want: want{
				mg:  newApp("docker", withExternalName(guid), withSpace(spaceGUID)),
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true, ResourceLateInitialized: true},
				err: nil,
			},
			service: func() *fake.MockApp {
				m := &fake.MockApp{}
				m.On("Get", guid).Return(
					fake.AppNil,
					fake.ErrNoResultReturned,
				)
				m.On("Single").Return(
					&fake.NewApp("docker").SetName(name).SetGUID(guid).App,
					nil,
				)
				return m
			},
			kube: &test.MockClient{},
		},
		"NotFound": {
			args: args{
				mg: newApp("docker", withExternalName(guid), withSpace(spaceGUID)),
			},
			want: want{
				mg:  newApp("docker", withExternalName(guid)),
				obs: managed.ExternalObservation{ResourceExists: false},
				err: nil,
			},
			service: func() *fake.MockApp {
				m := &fake.MockApp{}
				m.On("Get", guid).Return(
					fake.AppNil,
					fake.ErrNoResultReturned,
				)
				m.On("Single").Return(
					fake.AppNil,
					fake.ErrNoResultReturned,
				)
				return m
			},
			kube: &test.MockClient{},
		},
		"Successful": {
			args: args{
				mg: newApp("docker", withExternalName(guid), withSpace(spaceGUID)),
			},
			want: want{
				mg: newApp("docker",
					withExternalName(guid),
					withStatus(guid, "STARTED"),
					withConditions(xpv1.Available())),

				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
				err: nil,
			},
			service: func() *fake.MockApp {
				m := &fake.MockApp{}
				m.On("Get", guid).Return(
					&fake.NewApp("docker").SetName(name).SetGUID(guid).App,
					nil,
				)
				m.On("Single").Return(
					&fake.NewApp("docker").SetName(name).SetGUID(guid).App,
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
					MockUpdate: test.NewMockUpdateFn(nil),
				},
				client: &app.Client{
					AppClient:  tc.service(),
					PushClient: newMockPush(),
				},
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
	type service func() *fake.MockApp
	type job func() *fake.MockJob
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
		job
		kube k8s.Client
	}{
		"Successful": {
			args: args{
				mg: newApp("docker", withImage("docker-image"), withSpace(spaceGUID)),
			},
			want: want{
				mg: newApp("docker", withImage("docker-image"),
					withSpace(spaceGUID),
					withConditions(xpv1.Creating()),
					withExternalName(guid)),
				obs: managed.ExternalCreation{},
				err: nil,
			},
			service: func() *fake.MockApp {
				m := &fake.MockApp{}
				m.On("Create").Return(
					&fake.NewApp("docker").SetName(name).SetGUID(guid).App,
					nil,
				)
				m.On("Single").Return(
					&fake.NewApp("docker").SetName(name).SetGUID(guid).App,
					nil,
				)
				return m
			},
			job: func() *fake.MockJob {
				m := &fake.MockJob{}
				m.On("PollComplete").Return(nil)
				return m
			},
		},

		"AlreadyExist": {
			args: args{
				mg: newApp("docker", withSpace(spaceGUID), withImage("docker-image")),
			},
			want: want{
				mg: newApp("docker", withImage("docker-image"),
					withSpace(spaceGUID),
					withConditions(xpv1.Creating())),
				obs: managed.ExternalCreation{},
				err: errors.Wrap(errBoom, errCreateResource),
			},
			service: func() *fake.MockApp {
				m := &fake.MockApp{}
				m.On("Create").Return(
					fake.AppNil,
					errBoom,
				)
				m.On("Single").Return(
					&fake.NewApp("docker").SetName(name).SetGUID(guid).App,
					fake.ErrNoResultReturned,
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
				client: &app.Client{
					AppClient:  tc.service(),
					PushClient: newMockPush(),
				},
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

func TestUpdate(t *testing.T) {
	type service func() *fake.MockApp
	type job func() *fake.MockJob
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
		job
		kube k8s.Client
	}{
		"Successful": {
			args: args{
				mg: newApp("docker",
					withSpace(spaceGUID),
					withExternalName(guid),
					withStatus(guid, "STARTED")),
			},
			want: want{
				mg: newApp("docker",
					withSpace(spaceGUID),
					withExternalName(guid),
					withStatus(guid, "STARTED")),
				obs: managed.ExternalUpdate{},
				err: nil,
			},
			service: func() *fake.MockApp {
				m := &fake.MockApp{}
				m.On("Update", guid).Return(
					&fake.NewApp("docker").SetName(name).SetGUID(guid).App,
					nil,
				)
				m.On("Get", guid).Return(
					&fake.NewApp("docker").SetName(name).SetGUID(guid).App,
					nil,
				)
				return m
			},
		},

		"DoesNotExist": {
			args: args{
				mg: newApp("docker",
					withSpace(spaceGUID),
					withExternalName(guid),
					withStatus(guid, "STARTED")),
			},
			want: want{
				mg: newApp("docker",
					withSpace(spaceGUID),
					withExternalName(guid),
					withStatus(guid, "STARTED")),
				obs: managed.ExternalUpdate{},
				err: errors.Wrap(errBoom, errUpdateResource),
			},
			service: func() *fake.MockApp {
				m := &fake.MockApp{}
				m.On("Update", guid).Return(
					fake.AppNil,
					errBoom,
				)
				m.On("Get", guid).Return(
					fake.AppNil,
					errBoom,
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
				client: &app.Client{
					AppClient:  tc.service(),
					PushClient: newMockPush(),
				},
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
		})
	}
}
