package app

import (
	"context"
	"testing"

	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/pkg/errors"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/app"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/fake"
)

var (
	errBoom     = errors.New("boom")
	name        = "my-app"
	spaceGUID   = "a46808d1-d09a-4eef-add1-30872dec82f7"
	guid        = "2d8b0d04-d537-4e4e-8c6f-f09ca0e7f56f"
	envVarValue = "hello"
)

func assertErrAndObs[T any](t *testing.T, wantErr, gotErr error, wantObs, gotObs T) {
	t.Helper()
	if wantErr != nil && gotErr != nil {
		if diff := cmp.Diff(wantErr.Error(), gotErr.Error()); diff != "" {
			t.Errorf("want error string != got error string:\n%s", diff)
		}
	} else {
		if diff := cmp.Diff(wantErr, gotErr); diff != "" {
			t.Errorf("want error != got error:\n%s", diff)
		}
	}
	if diff := cmp.Diff(wantObs, gotObs); diff != "" {
		t.Errorf("-want, +got:\n%s", diff)
	}
}

type modifier func(*v1alpha1.App)

func withExternalName(name string) modifier {
	return func(r *v1alpha1.App) {
		r.Annotations[meta.AnnotationKeyExternalName] = name
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

func withRoutes(routes ...v1alpha1.AppRouteObservation) modifier {
	return func(r *v1alpha1.App) {
		r.Status.AtProvider.Routes = routes
	}
}

func withImage(image string) modifier {
	return func(r *v1alpha1.App) {
		r.Spec.ForProvider.Docker = &v1alpha1.DockerConfiguration{Image: image}
	}
}

func withEnvironment(env map[string]string) modifier {
	return func(r *v1alpha1.App) {
		r.Spec.ForProvider.Environment = env
	}
}

func withObservedName(n string) modifier {
	return func(r *v1alpha1.App) {
		r.Status.AtProvider.Name = n
	}
}

func withAppManifest(manifest string) modifier {
	return func(r *v1alpha1.App) {
		r.Status.AtProvider.AppManifest = manifest
	}
}

func withObservedLabels(labels map[string]*string) modifier {
	return func(r *v1alpha1.App) {
		r.Status.AtProvider.Labels = labels
	}
}

func withLabels(labels map[string]*string) modifier {
	return func(r *v1alpha1.App) {
		r.Spec.ForProvider.Labels = labels
	}
}

func newApp(typ string, m ...modifier) *v1alpha1.App {
	r := &v1alpha1.App{
		TypeMeta: metav1.TypeMeta{
			Kind:       v1alpha1.App_Kind,
			APIVersion: v1alpha1.CRDGroupVersion.String(),
		},
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
	m.On("GenerateManifest", guid).Return("applications:\n- name: "+name, nil)
	m.On("Push").Return(&fake.NewApp("docker").SetName(name).SetGUID(guid).App,
		nil,
	)
	return m

}

func withDefaultMetadataLabels() modifier {
	return func(r *v1alpha1.App) {
		r.SetGroupVersionKind(v1alpha1.App_GroupVersionKind)
	}
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
		args         args
		want         want
		service      service
		kube         k8s.Client
		routeFetcher *fake.MockRouteFetcher
		push         func() *fake.MockPush
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
		"AdoptionLookupFails": {
			args: args{
				mg: newApp("docker", withSpace(spaceGUID)),
			},
			want: want{
				mg:  newApp("docker", withSpace(spaceGUID)),
				obs: managed.ExternalObservation{},
				err: errors.Wrap(errBoom, errObserveResource),
			},
			service: func() *fake.MockApp {
				m := &fake.MockApp{}
				m.On("Single").Return(
					fake.AppNil,
					errBoom,
				)
				return m
			},
		},
		"Boom!": {
			args: args{
				mg: newApp("docker", withExternalName(guid), withSpace(spaceGUID)),
			},
			want: want{
				mg:  newApp("docker", withExternalName(guid), withSpace(spaceGUID)),
				obs: managed.ExternalObservation{},
				err: errors.Wrap(errBoom, errObserveResource),
			},
			service: func() *fake.MockApp {
				m := &fake.MockApp{}
				m.On("Get", guid).Return(
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
				obs: managed.ExternalObservation{ResourceExists: true, ResourceLateInitialized: true},
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
				mg:  newApp("docker", withExternalName(guid), withSpace(spaceGUID)),
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
				mg: newApp("docker", withExternalName(guid), withSpace(spaceGUID), withDefaultMetadataLabels()),
			},
			want: want{
				mg: newApp("docker",
					withExternalName(guid),
					withSpace(spaceGUID),
					withStatus(guid, "STARTED"),
					withObservedName(name),
					withAppManifest("applications:\n- name: "+name),
					withConditions(xpv1.Available()),
					withObservedLabels(map[string]*string{
						"crossplane-kind": ptr.To("app.cloudfoundry.crossplane.io"),
						"crossplane-name": ptr.To("my-app"),
					}),
				),

				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
				err: nil,
			},
			service: func() *fake.MockApp {
				m := &fake.MockApp{}
				m.On("Get", guid).Return(
					&fake.NewApp("docker").SetName(name).SetGUID(guid).SetLabels(map[string]*string{
						"crossplane-kind": ptr.To("app.cloudfoundry.crossplane.io"),
						"crossplane-name": ptr.To("my-app"),
					}).SetState("STARTED").App,
					nil,
				)
				m.On("Single").Return(
					&fake.NewApp("docker").SetName(name).SetGUID(guid).SetLabels(map[string]*string{
						"crossplane-kind": ptr.To("app.cloudfoundry.crossplane.io"),
						"crossplane-name": ptr.To("my-app"),
					}).SetState("STARTED").App,
					nil,
				)
				return m
			},
		},
		"RoutesPopulated": {
			args: args{
				mg: newApp("docker", withExternalName(guid), withSpace(spaceGUID), withDefaultMetadataLabels()),
			},
			want: want{
				mg: newApp("docker",
					withExternalName(guid),
					withSpace(spaceGUID),
					withStatus(guid, "STARTED"),
					withObservedName(name),
					withAppManifest("applications:\n- name: "+name),
					withRoutes(v1alpha1.AppRouteObservation{
						URL:      "myapp.apps.example.com",
						Host:     "myapp",
						Path:     "",
						Protocol: "http",
					}),
					withConditions(xpv1.Available()),
					withObservedLabels(map[string]*string{
						"crossplane-kind": ptr.To("app.cloudfoundry.crossplane.io"),
						"crossplane-name": ptr.To("my-app"),
					}),
				),
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
				err: nil,
			},
			service: func() *fake.MockApp {
				m := &fake.MockApp{}
				m.On("Get", guid).Return(
					&fake.NewApp("docker").SetName(name).SetGUID(guid).SetLabels(map[string]*string{
						"crossplane-kind": ptr.To("app.cloudfoundry.crossplane.io"),
						"crossplane-name": ptr.To("my-app"),
					}).SetState("STARTED").App,
					nil,
				)
				m.On("Single").Return(
					&fake.NewApp("docker").SetName(name).SetGUID(guid).SetLabels(map[string]*string{
						"crossplane-kind": ptr.To("app.cloudfoundry.crossplane.io"),
						"crossplane-name": ptr.To("my-app"),
					}).SetState("STARTED").App,
					nil,
				)
				return m
			},
			routeFetcher: func() *fake.MockRouteFetcher {
				m := &fake.MockRouteFetcher{}
				m.On("ListForAppAll", guid).Return(
					[]*cfresource.Route{
						{
							URL:      "myapp.apps.example.com",
							Host:     "myapp",
							Path:     "",
							Protocol: "http",
						},
					},
					nil,
				)
				return m
			}(),
		},
		"InvalidGUIDReturnsError": {
			args: args{
				mg: newApp("docker", withExternalName("not-a-guid"), withSpace(spaceGUID)),
			},
			want: want{
				mg:  newApp("docker", withExternalName("not-a-guid"), withSpace(spaceGUID)),
				obs: managed.ExternalObservation{},
				err: errors.Errorf("external-name 'not-a-guid' is not a valid GUID format"),
			},
			service: func() *fake.MockApp {
				return &fake.MockApp{}
			},
		},
		"Drift": {
			args: args{
				mg: newApp("docker", withExternalName(guid), withSpace(spaceGUID)),
			},
			want: want{
				mg: newApp("docker",
					withExternalName(guid),
					withSpace(spaceGUID),
					withStatus(guid, "STARTED"),
					withObservedName("other-name"),
					withAppManifest("applications:\n- name: other-name"),
					withConditions(xpv1.Available())),
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: false},
				err: nil,
			},
			service: func() *fake.MockApp {
				m := &fake.MockApp{}
				m.On("Get", guid).Return(
					&fake.NewApp("docker").SetName("other-name").SetGUID(guid).SetState("STARTED").App,
					nil,
				)
				return m
			},
			push: func() *fake.MockPush {
				m := &fake.MockPush{}
				m.On("GenerateManifest", guid).Return("applications:\n- name: other-name", nil)
				return m
			},
		},
		"RouteFetchErrorNonFatal": {
			args: args{
				mg: newApp("docker", withExternalName(guid), withSpace(spaceGUID), withDefaultMetadataLabels(),
					withRoutes(v1alpha1.AppRouteObservation{
						URL:      "stale.apps.example.com",
						Host:     "stale",
						Protocol: "http",
					})),
			},
			want: want{
				mg: newApp("docker",
					withExternalName(guid),
					withSpace(spaceGUID),
					withStatus(guid, "STARTED"),
					withObservedName(name),
					withAppManifest("applications:\n- name: "+name),
					withRoutes(v1alpha1.AppRouteObservation{
						URL:      "stale.apps.example.com",
						Host:     "stale",
						Protocol: "http",
					}),
					withConditions(xpv1.Available()),
					withObservedLabels(map[string]*string{
						"crossplane-kind": ptr.To("app.cloudfoundry.crossplane.io"),
						"crossplane-name": ptr.To("my-app"),
					}),
				),
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
				err: nil,
			},
			service: func() *fake.MockApp {
				m := &fake.MockApp{}
				m.On("Get", guid).Return(
					&fake.NewApp("docker").SetName(name).SetGUID(guid).SetLabels(map[string]*string{
						"crossplane-kind": ptr.To("app.cloudfoundry.crossplane.io"),
						"crossplane-name": ptr.To("my-app"),
					}).SetState("STARTED").App,
					nil,
				)
				m.On("Single").Return(
					&fake.NewApp("docker").SetName(name).SetGUID(guid).SetLabels(map[string]*string{
						"crossplane-kind": ptr.To("app.cloudfoundry.crossplane.io"),
						"crossplane-name": ptr.To("my-app"),
					}).SetState("STARTED").App,
					nil,
				)
				return m
			},
			routeFetcher: func() *fake.MockRouteFetcher {
				m := &fake.MockRouteFetcher{}
				m.On("ListForAppAll", guid).Return(
					([]*cfresource.Route)(nil),
					errBoom,
				)
				return m
			}(),
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
					AppClient: tc.service(),
					PushClient: func() *fake.MockPush {
						if tc.push != nil {
							return tc.push()
						}
						return newMockPush()
					}(),
				},
			}
			if tc.routeFetcher != nil {
				c.client.RouteFetcher = tc.routeFetcher
			}

			obs, err := c.Observe(context.Background(), tc.args.mg)

			assertErrAndObs(t, tc.want.err, err, tc.want.obs, obs)

			if diff := cmp.Diff(tc.want.mg, tc.args.mg,
				cmpopts.IgnoreFields(v1alpha1.Resource{}, "CreatedAt", "UpdatedAt"),
			); diff != "" {
				t.Errorf("Observe(...): -want mg, +got mg:\n%s", diff)
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

		"CreateFails": {
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
				m.On("Create").Return(fake.AppNil, errBoom)
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

			assertErrAndObs(t, tc.want.err, err, tc.want.obs, obs)
			if diff := cmp.Diff(tc.want.mg, tc.args.mg); diff != "" {
				t.Errorf("Create(...): -want, +got:\n%s", diff)
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
		push    func() *fake.MockPush
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
				return m
			},
		},

		"EnvVarAdded": {
			args: args{
				mg: newApp("docker",
					withSpace(spaceGUID),
					withExternalName(guid),
					withStatus(guid, "STARTED"),
					withObservedName(name),
					withEnvironment(map[string]string{"MY_VAR": envVarValue})),
			},
			want: want{
				mg: newApp("docker",
					withSpace(spaceGUID),
					withExternalName(guid),
					withStatus(guid, "STARTED"),
					withObservedName(name),
					withEnvironment(map[string]string{"MY_VAR": envVarValue})),
				obs: managed.ExternalUpdate{},
				err: nil,
			},
			service: func() *fake.MockApp {
				m := &fake.MockApp{}
				v := envVarValue
				m.On("GetEnvironmentVariables", guid).Return(map[string]*string{}, nil)
				m.On("SetEnvironmentVariables", guid, map[string]*string{"MY_VAR": &v}).Return(map[string]*string{}, nil)
				m.On("Stop", guid).Return(&fake.NewApp("docker").SetName(name).SetGUID(guid).App, nil)
				m.On("Start", guid).Return(&fake.NewApp("docker").SetName(name).SetGUID(guid).App, nil)
				m.On("Update", guid).Return(&fake.NewApp("docker").SetName(name).SetGUID(guid).App, nil)
				return m
			},
		},

		"EnvVarDeleted": {
			args: args{
				mg: newApp("docker",
					withSpace(spaceGUID),
					withExternalName(guid),
					withStatus(guid, "STARTED"),
					withObservedName(name),
					withAppManifest("applications:\n- name: "+name+"\n  env:\n    ANOTHER_VAR: world")),
			},
			want: want{
				mg: newApp("docker",
					withSpace(spaceGUID),
					withExternalName(guid),
					withStatus(guid, "STARTED"),
					withObservedName(name),
					withAppManifest("applications:\n- name: "+name+"\n  env:\n    ANOTHER_VAR: world")),
				obs: managed.ExternalUpdate{},
				err: nil,
			},
			service: func() *fake.MockApp {
				m := &fake.MockApp{}
				world := "world"
				m.On("GetEnvironmentVariables", guid).Return(map[string]*string{"ANOTHER_VAR": &world}, nil)
				m.On("SetEnvironmentVariables", guid, map[string]*string{"ANOTHER_VAR": (*string)(nil)}).Return(map[string]*string{}, nil)
				m.On("Stop", guid).Return(&fake.NewApp("docker").SetName(name).SetGUID(guid).App, nil)
				m.On("Start", guid).Return(&fake.NewApp("docker").SetName(name).SetGUID(guid).App, nil)
				m.On("Update", guid).Return(&fake.NewApp("docker").SetName(name).SetGUID(guid).App, nil)
				return m
			},
		},

		"EnvVarGetFails": {
			args: args{
				mg: newApp("docker",
					withSpace(spaceGUID),
					withExternalName(guid),
					withStatus(guid, "STARTED"),
					withObservedName(name),
					withEnvironment(map[string]string{"MY_VAR": envVarValue})),
			},
			want: want{
				mg: newApp("docker",
					withSpace(spaceGUID),
					withExternalName(guid),
					withStatus(guid, "STARTED"),
					withObservedName(name),
					withEnvironment(map[string]string{"MY_VAR": envVarValue})),
				obs: managed.ExternalUpdate{},
				err: errors.Wrap(errBoom, errUpdateResource),
			},
			service: func() *fake.MockApp {
				m := &fake.MockApp{}
				m.On("GetEnvironmentVariables", guid).Return(nil, errBoom)
				return m
			},
		},

		"EnvVarUpdated_AppStopped": {
			args: args{
				mg: newApp("docker",
					withSpace(spaceGUID),
					withExternalName(guid),
					withStatus(guid, "STOPPED"),
					withObservedName(name),
					withEnvironment(map[string]string{"MY_VAR": envVarValue})),
			},
			want: want{
				mg: newApp("docker",
					withSpace(spaceGUID),
					withExternalName(guid),
					withStatus(guid, "STOPPED"),
					withObservedName(name),
					withEnvironment(map[string]string{"MY_VAR": envVarValue})),
				obs: managed.ExternalUpdate{},
				err: nil,
			},
			service: func() *fake.MockApp {
				m := &fake.MockApp{}
				v := envVarValue
				m.On("GetEnvironmentVariables", guid).Return(map[string]*string{}, nil)
				m.On("SetEnvironmentVariables", guid, map[string]*string{"MY_VAR": &v}).Return(map[string]*string{}, nil)
				// No Stop/Start expected — app is already stopped
				m.On("Update", guid).Return(&fake.NewApp("docker").SetName(name).SetGUID(guid).App, nil)
				return m
			},
		},

		"MetadataOnly": {
			args: args{
				mg: newApp("docker",
					withSpace(spaceGUID),
					withExternalName(guid),
					withStatus(guid, "STARTED"),
					withObservedName(name),
					withLabels(map[string]*string{"env": ptr.To("prod")}),
					withObservedLabels(map[string]*string{"env": ptr.To("dev")})),
			},
			want: want{
				mg: newApp("docker",
					withSpace(spaceGUID),
					withExternalName(guid),
					withStatus(guid, "STARTED"),
					withObservedName(name),
					withLabels(map[string]*string{"env": ptr.To("prod")}),
					withObservedLabels(map[string]*string{"env": ptr.To("dev")})),
				obs: managed.ExternalUpdate{},
				err: nil,
			},
			service: func() *fake.MockApp {
				m := &fake.MockApp{}
				m.On("Update", guid).Return(
					&fake.NewApp("docker").SetName(name).SetGUID(guid).App,
					nil,
				)
				return m
			},
		},

		"EnvVarAndMetadataChanged": {
			args: args{
				mg: newApp("docker",
					withSpace(spaceGUID),
					withExternalName(guid),
					withStatus(guid, "STARTED"),
					withObservedName(name),
					withEnvironment(map[string]string{"MY_VAR": envVarValue}),
					withLabels(map[string]*string{"env": ptr.To("prod")}),
					withObservedLabels(map[string]*string{"env": ptr.To("dev")})),
			},
			want: want{
				mg: newApp("docker",
					withSpace(spaceGUID),
					withExternalName(guid),
					withStatus(guid, "STARTED"),
					withObservedName(name),
					withEnvironment(map[string]string{"MY_VAR": envVarValue}),
					withLabels(map[string]*string{"env": ptr.To("prod")}),
					withObservedLabels(map[string]*string{"env": ptr.To("dev")})),
				obs: managed.ExternalUpdate{},
				err: nil,
			},
			service: func() *fake.MockApp {
				m := &fake.MockApp{}
				v := envVarValue
				m.On("GetEnvironmentVariables", guid).Return(map[string]*string{}, nil)
				m.On("SetEnvironmentVariables", guid, map[string]*string{"MY_VAR": &v}).Return(map[string]*string{}, nil)
				m.On("Stop", guid).Return(&fake.NewApp("docker").SetName(name).SetGUID(guid).App, nil)
				m.On("Start", guid).Return(&fake.NewApp("docker").SetName(name).SetGUID(guid).App, nil)
				m.On("Update", guid).Return(&fake.NewApp("docker").SetName(name).SetGUID(guid).App, nil)
				return m
			},
		},

		"EnvVarAndDockerChanged": {
			args: args{
				mg: newApp("docker",
					withSpace(spaceGUID),
					withExternalName(guid),
					withStatus(guid, "STARTED"),
					withObservedName(name),
					withAppManifest("applications:\n- name: "+name+"\n  docker:\n    image: old-image:v1"),
					withImage("new-image:v2"),
					withEnvironment(map[string]string{"MY_VAR": envVarValue})),
			},
			want: want{
				mg: newApp("docker",
					withSpace(spaceGUID),
					withExternalName(guid),
					withStatus(guid, "STARTED"),
					withObservedName(name),
					withAppManifest("applications:\n- name: "+name+"\n  docker:\n    image: old-image:v1"),
					withImage("new-image:v2"),
					withEnvironment(map[string]string{"MY_VAR": envVarValue})),
				obs: managed.ExternalUpdate{},
				err: nil,
			},
			push: func() *fake.MockPush {
				m := &fake.MockPush{}
				// Push is called during UpdateAndPush; return the app with the new image applied
				m.On("Push").Return(&fake.NewApp("docker").SetName(name).SetGUID(guid).App, nil)
				return m
			},
			service: func() *fake.MockApp {
				m := &fake.MockApp{}
				v := envVarValue
				// UpdateAndPush (docker image update) calls AppClient.Update
				m.On("Update", guid).Return(&fake.NewApp("docker").SetName(name).SetGUID(guid).App, nil)
				m.On("GetEnvironmentVariables", guid).Return(map[string]*string{}, nil)
				m.On("SetEnvironmentVariables", guid, map[string]*string{"MY_VAR": &v}).Return(map[string]*string{}, nil)
				// No Stop/Start expected — docker push already restarted the app
				return m
			},
		},

		"EnvVarUpdate_StopFails": {
			args: args{
				mg: newApp("docker",
					withSpace(spaceGUID),
					withExternalName(guid),
					withStatus(guid, "STARTED"),
					withObservedName(name),
					withEnvironment(map[string]string{"MY_VAR": envVarValue})),
			},
			want: want{
				mg: newApp("docker",
					withSpace(spaceGUID),
					withExternalName(guid),
					withStatus(guid, "STARTED"),
					withObservedName(name),
					withEnvironment(map[string]string{"MY_VAR": envVarValue})),
				obs: managed.ExternalUpdate{},
				err: errors.Wrap(errBoom, errUpdateResource),
			},
			service: func() *fake.MockApp {
				m := &fake.MockApp{}
				v := envVarValue
				m.On("GetEnvironmentVariables", guid).Return(map[string]*string{}, nil)
				m.On("SetEnvironmentVariables", guid, map[string]*string{"MY_VAR": &v}).Return(map[string]*string{}, nil)
				m.On("Stop", guid).Return(nil, errBoom)
				return m
			},
		},

		"EnvVarUpdate_StartFails": {
			args: args{
				mg: newApp("docker",
					withSpace(spaceGUID),
					withExternalName(guid),
					withStatus(guid, "STARTED"),
					withObservedName(name),
					withEnvironment(map[string]string{"MY_VAR": envVarValue})),
			},
			want: want{
				mg: newApp("docker",
					withSpace(spaceGUID),
					withExternalName(guid),
					withStatus(guid, "STARTED"),
					withObservedName(name),
					withEnvironment(map[string]string{"MY_VAR": envVarValue})),
				obs: managed.ExternalUpdate{},
				err: errors.Wrap(errBoom, errUpdateResource),
			},
			service: func() *fake.MockApp {
				m := &fake.MockApp{}
				v := envVarValue
				m.On("GetEnvironmentVariables", guid).Return(map[string]*string{}, nil)
				m.On("SetEnvironmentVariables", guid, map[string]*string{"MY_VAR": &v}).Return(map[string]*string{}, nil)
				m.On("Stop", guid).Return(&fake.NewApp("docker").SetName(name).SetGUID(guid).App, nil)
				m.On("Start", guid).Return(nil, errBoom)
				return m
			},
		},

		"EmptyGUID": {
			args: args{
				mg: newApp("docker", withSpace(spaceGUID)),
			},
			want: want{
				mg:  newApp("docker", withSpace(spaceGUID)),
				obs: managed.ExternalUpdate{},
				err: nil,
			},
			service: func() *fake.MockApp {
				return &fake.MockApp{}
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			t.Logf("Testing: %s", t.Name())
			mockApp := tc.service()
			pushMock := newMockPush()
			if tc.push != nil {
				pushMock = tc.push()
			}
			c := &external{
				kube: &test.MockClient{
					MockUpdate:       test.NewMockUpdateFn(nil),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				client: &app.Client{
					AppClient:  mockApp,
					PushClient: pushMock,
				},
			}

			obs, err := c.Update(context.Background(), tc.args.mg)

			assertErrAndObs(t, tc.want.err, err, tc.want.obs, obs)
			if diff := cmp.Diff(tc.want.mg, tc.args.mg); diff != "" {
				t.Errorf("Update(...): -want, +got:\n%s", diff)
			}
			mockApp.AssertExpectations(t)
			if tc.push != nil {
				pushMock.AssertExpectations(t)
			}
		})
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
		args    args
		want    want
		service func() *fake.MockApp
		job     func() *fake.MockJob
	}{
		"Successful": {
			args: args{
				mg: newApp("docker", withExternalName(guid), withSpace(spaceGUID)),
			},
			want: want{
				mg:  newApp("docker", withExternalName(guid), withSpace(spaceGUID), withConditions(xpv1.Deleting())),
				obs: managed.ExternalDelete{},
				err: nil,
			},
			service: func() *fake.MockApp {
				m := &fake.MockApp{}
				m.On("Delete", guid).Return("job-guid", nil)
				return m
			},
			job: func() *fake.MockJob {
				m := &fake.MockJob{}
				m.On("PollComplete").Return(nil)
				return m
			},
		},

		"NotFound": {
			args: args{
				mg: newApp("docker", withExternalName(guid), withSpace(spaceGUID)),
			},
			want: want{
				mg:  newApp("docker", withExternalName(guid), withSpace(spaceGUID), withConditions(xpv1.Deleting())),
				obs: managed.ExternalDelete{},
				err: nil,
			},
			service: func() *fake.MockApp {
				m := &fake.MockApp{}
				m.On("Delete", guid).Return("", fake.ErrNoResultReturned)
				return m
			},
			job: func() *fake.MockJob {
				return &fake.MockJob{}
			},
		},

		"EmptyGUID": {
			args: args{
				mg: newApp("docker", withSpace(spaceGUID)),
			},
			want: want{
				mg:  newApp("docker", withSpace(spaceGUID), withConditions(xpv1.Deleting())),
				obs: managed.ExternalDelete{},
				err: nil,
			},
			service: func() *fake.MockApp {
				return &fake.MockApp{}
			},
			job: func() *fake.MockJob {
				return &fake.MockJob{}
			},
		},

		"DeleteFails": {
			args: args{
				mg: newApp("docker", withExternalName(guid), withSpace(spaceGUID)),
			},
			want: want{
				mg:  newApp("docker", withExternalName(guid), withSpace(spaceGUID), withConditions(xpv1.Deleting())),
				obs: managed.ExternalDelete{},
				err: errors.Wrap(errBoom, errDeleteResource),
			},
			service: func() *fake.MockApp {
				m := &fake.MockApp{}
				m.On("Delete", guid).Return("", errBoom)
				return m
			},
			job: func() *fake.MockJob {
				return &fake.MockJob{}
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
					AppClient: tc.service(),
					Job:       tc.job(),
				},
			}

			obs, err := c.Delete(context.Background(), tc.args.mg)

			assertErrAndObs(t, tc.want.err, err, tc.want.obs, obs)
			if diff := cmp.Diff(tc.want.mg, tc.args.mg); diff != "" {
				t.Errorf("Delete(...): -want mg, +got mg:\n%s", diff)
			}
		})
	}
}
