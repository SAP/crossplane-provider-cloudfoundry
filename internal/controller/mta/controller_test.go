package mta

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	mtaModels "github.com/cloudfoundry-incubator/multiapps-cli-plugin/clients/models"
	mtaClient "github.com/cloudfoundry-incubator/multiapps-cli-plugin/clients/mtaclient"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/fake"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/mta"
)

var (
	errBoom           = errors.New("boom")
	errAborted        = errors.New("ABORTED")
	name              = "my-mta"
	spaceGUID         = "a46808d1-d09a-4eef-add1-30872dec82f7"
	mtaGUID           = "2d8b0d04-d537-4e4e-8c6f-f09ca0e7f56f"
	emptyString       = ""
	headerNil         http.Header
	MtaObservationNil *mtaModels.Operation
	MtaModelNil       *mtaModels.Mta
)

type modifier func(*v1alpha1.Mta)

func withSpace(space string) modifier {
	return func(r *v1alpha1.Mta) {
		r.Spec.ForProvider.Space = &space
	}
}

func withFile(fileName string) modifier {
	return func(r *v1alpha1.Mta) {
		r.Status.AtProvider.Files = &[]v1alpha1.FileObservation{{
			ID:            &emptyString,
			AppInstance:   &emptyString,
			URL:           &fileName,
			LastOperation: &v1alpha1.Operation{ID: &emptyString}},
		}
	}
}

func withConditions(c ...xpv1.Condition) modifier {
	return func(i *v1alpha1.Mta) { i.Status.SetConditions(c...) }
}

func withLastOperation(id string) modifier {
	return func(r *v1alpha1.Mta) {
		r.Status.AtProvider.LastOperation = &v1alpha1.Operation{
			ID: &id,
		}
	}
}

func resetFileId() modifier {
	return func(r *v1alpha1.Mta) {
		if r.Status.AtProvider.Files != nil && len(*r.Status.AtProvider.Files) > 0 {
			(*r.Status.AtProvider.Files)[0].ID = nil
		}
	}
}

func withId(mtaGUID string) modifier {
	return func(r *v1alpha1.Mta) {
		r.Status.AtProvider.MtaId = &mtaGUID
	}
}

func withUrl(image string) modifier {
	return func(r *v1alpha1.Mta) {
		r.Spec.ForProvider.File = &v1alpha1.File{URL: &image}
	}
}

func withAbortOnError(value bool) modifier {
	return func(r *v1alpha1.Mta) {
		r.Spec.ForProvider.AbortOnError = &value
	}
}

func withVersionRule(value string) modifier {
	return func(r *v1alpha1.Mta) {
		r.Spec.ForProvider.VersionRule = &value
	}
}

func withModules(value []string) modifier {
	return func(r *v1alpha1.Mta) {
		r.Spec.ForProvider.Modules = &value
	}
}

func newMta(m ...modifier) *v1alpha1.Mta {
	r := &v1alpha1.Mta{

		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Finalizers:  []string{},
			Annotations: map[string]string{},
		},
		Spec: v1alpha1.MtaSpec{
			ForProvider: v1alpha1.MtaParameters{},
		},
		Status: v1alpha1.MtaStatus{
			AtProvider: v1alpha1.MtaObservation{},
		},
	}

	for _, rm := range m {
		rm(r)
	}
	return r
}

func TestCreate(t *testing.T) {
	type service func() *fake.MockMTA
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
				mg: newMta(withUrl("mtar-file"), withSpace(spaceGUID)),
			},
			want: want{
				mg: newMta(withUrl("mtar-file"),
					withSpace(spaceGUID),
					withConditions(xpv1.Creating()),
					withFile("mtar-file"),
					resetFileId()),
				obs: managed.ExternalCreation{ConnectionDetails: managed.ConnectionDetails{}},
				err: nil,
			},
			service: func() *fake.MockMTA {
				m := &fake.MockMTA{}
				m.On("StartMtaOperation").Return(
					&fake.NewMta().SetMetadataID(mtaGUID).Mta,
					nil,
				)
				m.On("StartUploadMtaArchiveFromUrl").Return(
					http.Header{},
					nil,
				)
				return m
			},
		},
		"CorruptedFileProvided": {
			args: args{
				mg: newMta(withUrl("CORRUPTED"), withSpace(spaceGUID)),
			},
			want: want{
				mg: newMta(
					withSpace(spaceGUID),
					withConditions(xpv1.Creating()),
					withUrl("CORRUPTED"),
				),
				obs: managed.ExternalCreation{ConnectionDetails: nil},
				err: errors.Wrap(errors.Wrap(errBoom, errCreate), errCreateFile),
			},
			service: func() *fake.MockMTA {
				m := &fake.MockMTA{}
				m.On("StartUploadMtaArchiveFromUrl").Return(
					headerNil,
					errBoom,
				)
				return m
			},
		},
		"FaultyDeployStep": {
			args: args{
				mg: newMta(withUrl("mtar-file"), withSpace(spaceGUID), withFile("mtar-file"), withId(mtaGUID)),
			},
			want: want{
				mg: newMta(withUrl("mtar-file"),
					withSpace(spaceGUID),
					withConditions(xpv1.Creating()),
					withFile("mtar-file"),
					withId(mtaGUID)),
				obs: managed.ExternalCreation{ConnectionDetails: nil},
				err: errors.Wrap(errBoom, errCreate),
			},
			service: func() *fake.MockMTA {
				m := &fake.MockMTA{}
				m.On("StartMtaOperation").Return(
					mtaClient.ResponseHeader{},
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
				client: mta.Client{
					MtaClient: tc.service(),
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

func TestObserve(t *testing.T) {
	type service func() *fake.MockMTA
	type args struct {
		mg resource.Managed
	}

	type want struct {
		mg  *v1alpha1.Mta
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
				err: errors.New(errNotMta),
			},
			service: func() *fake.MockMTA {
				m := &fake.MockMTA{}
				return m
			},
		},

		// This tests whether the external API is reachable

		"Error when external API is not working": {
			args: args{
				mg: newMta(withLastOperation(mtaGUID)),
			},
			want: want{
				mg:  newMta(withLastOperation(mtaGUID)),
				obs: managed.ExternalObservation{},
				err: errors.Wrap(errBoom, errGet),
			},
			service: func() *fake.MockMTA {
				m := &fake.MockMTA{}
				m.On("GetMtaOperation").Return(
					MtaObservationNil,
					errBoom,
				)

				return m
			},
		},
		"Mta with guid does not exist": {
			args: args{
				mg: newMta(
					withLastOperation(mtaGUID),
					withFile("mtar-file"),
					withId(mtaGUID),
				),
			},
			want: want{
				mg: newMta(
					withLastOperation(mtaGUID),
					withFile("mtar-file"),
				),
				obs: managed.ExternalObservation{ResourceExists: false, ResourceLateInitialized: false},
				err: errBoom,
			},
			service: func() *fake.MockMTA {
				m := &fake.MockMTA{}
				m.On("GetMtaOperation").Return(
					&mtaModels.Operation{State: mtaModels.StateERROR},
					nil,
				)
				m.On("GetAsyncUploadJob").Return(
					mtaClient.AsyncUploadJobResult{Status: "", Error: "", MtaId: ""},
					nil,
				)
				m.On("GetMta").Return(
					MtaModelNil,
					errBoom,
				)
				return m
			},
			kube: &test.MockClient{},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			c := &external{
				kube: &test.MockClient{
					MockUpdate:       test.NewMockUpdateFn(nil),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				client: mta.Client{
					MtaClient: tc.service(),
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

func TestDelete(t *testing.T) {
	type service func() *fake.MockMTA
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
		"SuccessfulDeletion": {
			args: args{
				mg: newMta(withUrl("mtar-file"), withSpace(spaceGUID)),
			},
			want: want{
				mg:  newMta(withUrl("mtar-file"), withSpace(spaceGUID)),
				err: nil,
			},
			service: func() *fake.MockMTA {
				m := &fake.MockMTA{}
				m.On("StartMtaOperation").Return(
					mtaClient.ResponseHeader{},
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
				client: mta.Client{
					MtaClient: tc.service(),
				},
			}

			err := c.Delete(context.Background(), tc.args.mg)

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

func TestAbortOnError(t *testing.T) {
	type service func() *fake.MockMTA
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
		"SuccessfulDeployment_AbortOnError_True": {
			args: args{
				mg: newMta(
					withAbortOnError(true),
					withUrl("mtar-file"),
				),
			},
			want: want{
				mg: newMta(
					withAbortOnError(true),
					withUrl("mtar-file"),
				),
				obs: managed.ExternalObservation{ResourceExists: true, ResourceLateInitialized: true},
				err: nil,
			},
			service: func() *fake.MockMTA {
				m := &fake.MockMTA{}
				m.On("GetMta").Return(
					&fake.NewMta().SetMetadataID(mtaGUID).Mta,
					nil,
				)
				m.On("StartUploadMtaArchiveFromUrl").Return(
					http.Header{},
					nil,
				)
				return m
			},
			kube: &test.MockClient{},
		},
		"FailedDeployment_AbortOnError_True": {
			args: args{
				mg: newMta(
					withAbortOnError(true),
					withUrl("mtar-file"),
				),
			},
			want: want{
				mg: newMta(
					withAbortOnError(true),
					withUrl("mtar-file"),
				),
				obs: managed.ExternalObservation{},
				err: errAborted,
			},
			service: func() *fake.MockMTA {
				m := &fake.MockMTA{}
				m.On("GetMta").Return(
					nil,
					errAborted,
				)
				m.On("StartUploadMtaArchiveFromUrl").Return(
					headerNil,
					nil,
				)
				return m
			},
			kube: &test.MockClient{},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			t.Logf("Testing: %s", n)

			c := &external{
				kube: &test.MockClient{
					MockUpdate:       test.NewMockUpdateFn(nil),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				client: mta.Client{
					MtaClient: tc.service(),
				},
			}

			obs, err := c.Create(context.Background(), tc.args.mg)
			t.Logf("Create call: obs=%+v, err=%+v", obs, err)

			if tc.want.err != nil && err != nil {
				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("Create(...): want error string != got error string:\n%s", diff)
				} else {
					if diff := cmp.Diff(tc.want.err, err); diff != "" {
						t.Errorf("Create(...): want error != got error:\n%s", diff)
					}
				}
				if diff := cmp.Diff(tc.want.mg, tc.args.mg); diff != "" {
					t.Errorf("Create(...): -want, +got:\n%s", diff)
				}
			}
		})
	}
}

func TestVersionRule(t *testing.T) {
	type service func() *fake.MockMTA
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
		"SuccessfulDeployment_VersionRule_SAME_HIGHER": {
			args: args{
				mg: newMta(
					withVersionRule("SAME_HIGHER"),
					withUrl("mtar-file"),
				),
			},
			want: want{
				mg: newMta(
					withVersionRule("SAME_HIGHER"),
					withUrl("mtar-file"),
				),
				obs: managed.ExternalObservation{ResourceExists: true, ResourceLateInitialized: true},
				err: nil,
			},
			service: func() *fake.MockMTA {
				m := &fake.MockMTA{}
				m.On("GetMta").Return(
					&fake.NewMta().SetMetadataID(mtaGUID).Mta,
					nil,
				)
				m.On("StartUploadMtaArchiveFromUrl").Return(
					http.Header{},
					nil,
				)
				return m
			},
			kube: &test.MockClient{},
		},
		"FailedDeployment_VersionRule_Lower": {
			args: args{
				mg: newMta(
					withVersionRule("LOWER"),
					withUrl("mtar-file"),
				),
			},
			want: want{
				mg: newMta(
					withVersionRule("LOWER"),
					withUrl("mtar-file"),
				),
				obs: managed.ExternalObservation{ResourceExists: false, ResourceLateInitialized: false},
				err: errors.Wrap(errBoom, errCreate),
			},
			service: func() *fake.MockMTA {
				m := &fake.MockMTA{}
				m.On("GetMta").Return(
					MtaModelNil,
					errors.Wrap(errBoom, errCreate),
				)
				m.On("StartUploadMtaArchiveFromUrl").Return(
					headerNil,
					nil,
				)
				return m
			},
			kube: &test.MockClient{},
		},
	}
	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			t.Logf("Testing: %s", n)

			c := &external{
				kube: &test.MockClient{
					MockUpdate:       test.NewMockUpdateFn(nil),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				client: mta.Client{
					MtaClient: tc.service(),
				},
			}

			obs, err := c.Create(context.Background(), tc.args.mg)
			t.Logf("Create call: obs=%+v, err=%+v", obs, err)

			if tc.want.err != nil && err != nil {
				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("Create(...): want error string != got error string:\n%s", diff)
				} else {
					if diff := cmp.Diff(tc.want.err, err); diff != "" {
						t.Errorf("Create(...): want error != got error:\n%s", diff)
					}
				}
				if diff := cmp.Diff(tc.want.mg, tc.args.mg); diff != "" {
					t.Errorf("Create(...): -want, +got:\n%s", diff)
				}
			}
		})
	}
}

// TestModules ensures that only the specified modules in an MTA are processed and created correctly.
func TestModules(t *testing.T) {
	type service func() *fake.MockMTA
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
		"SuccessfulDeployment_WithEmptyModules": {
			args: args{
				mg: newMta(
					withModules([]string{}),
					withUrl("mtar-file"),
				),
			},
			want: want{
				mg: newMta(
					withModules([]string{}),
					withUrl("mtar-file"),
				),
				obs: managed.ExternalObservation{ResourceExists: true, ResourceLateInitialized: true},
				err: nil,
			},
			service: func() *fake.MockMTA {
				m := &fake.MockMTA{}
				m.On("GetMta").Return(
					&fake.NewMta().SetMetadataID(mtaGUID).Mta,
					nil,
				)
				m.On("StartUploadMtaArchiveFromUrl").Return(
					http.Header{},
					nil,
				)
				return m
			},
			kube: &test.MockClient{},
		},
		"SuccessfulDeployment_WithOneModule": {
			args: args{
				mg: newMta(
					withModules([]string{"bookshkop-srv"}),
					withUrl("mtar-file"),
				),
			},
			want: want{
				mg: newMta(
					withModules([]string{"bookshkop-srv"}),
					withUrl("mtar-file"),
				),
				obs: managed.ExternalObservation{ResourceExists: true, ResourceLateInitialized: true},
				err: nil,
			},
			service: func() *fake.MockMTA {
				m := &fake.MockMTA{}
				m.On("GetMta").Return(
					&fake.NewMta().SetMetadataID(mtaGUID).Mta,
					nil,
				)
				m.On("StartUploadMtaArchiveFromUrl").Return(
					http.Header{},
					nil,
				)
				return m
			},
			kube: &test.MockClient{},
		},
		"SuccessfulDeployment_WithMultipleModules": {
			args: args{
				mg: newMta(
					withModules([]string{"bookshkop-srv", "bookshkop-srv-module2"}),
					withUrl("mtar-file"),
				),
			},
			want: want{
				mg: newMta(
					withModules([]string{"bookshkop-srv", "bookshkop-srv-module2"}),
					withUrl("mtar-file"),
				),
				obs: managed.ExternalObservation{ResourceExists: true, ResourceLateInitialized: true},
				err: nil,
			},
			service: func() *fake.MockMTA {
				m := &fake.MockMTA{}
				m.On("GetMta").Return(
					&fake.NewMta().SetMetadataID(mtaGUID).Mta,
					nil,
				)
				m.On("StartUploadMtaArchiveFromUrl").Return(
					http.Header{},
					nil,
				)
				return m
			},
			kube: &test.MockClient{},
		},
		"InvalidNameInput": {
			args: args{
				mg: newMta(
					withModules([]string{"bookshkop-srv", "bookshkop-srv-module2", "INVALID-NAME"}),
					withUrl("mtar-file"),
				),
			},
			want: want{
				mg: newMta(
					withModules([]string{"bookshkop-srv", "bookshkop-srv-module2"}),
					withUrl("mtar-file"),
				),
				obs: managed.ExternalObservation{ResourceExists: false, ResourceLateInitialized: false},
				err: errors.Wrap(errBoom, errCreate),
			},
			service: func() *fake.MockMTA {
				m := &fake.MockMTA{}
				m.On("GetMta").Return(
					MtaModelNil,
					errors.Wrap(errBoom, errCreate),
				)
				m.On("StartUploadMtaArchiveFromUrl").Return(
					headerNil,
					nil,
				)
				return m
			},
		},
		"DuplicateModulesNotAllowed": {
			args: args{
				mg: newMta(
					withModules([]string{"DUPLICATE", "DUPLICATE"}),
					withUrl("mtar-file"),
				),
			},
			want: want{
				mg: newMta(
					withModules([]string{"DUPLICATE", "DUPLICATE"}),
					withUrl("mtar-file"),
				),
				obs: managed.ExternalObservation{ResourceExists: false, ResourceLateInitialized: false},
				err: errors.Wrap(errBoom, errCreate),
			},
			service: func() *fake.MockMTA {
				m := &fake.MockMTA{}
				m.On("GetMta").Return(
					MtaModelNil,
					errors.Wrap(errBoom, errCreate),
				)
				m.On("StartUploadMtaArchiveFromUrl").Return(
					headerNil,
					nil,
				)
				return m
			},
		},
	}
	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			t.Logf("Testing: %s", n)

			c := &external{
				kube: &test.MockClient{
					MockUpdate:       test.NewMockUpdateFn(nil),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				client: mta.Client{
					MtaClient: tc.service(),
				},
			}

			obs, err := c.Create(context.Background(), tc.args.mg)
			t.Logf("Create call: obs=%+v, err=%+v", obs, err)

			if tc.want.err != nil && err != nil {
				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("Create(...): want error string != got error string:\n%s", diff)
				} else {
					if diff := cmp.Diff(tc.want.err, err); diff != "" {
						t.Errorf("Create(...): want error != got error:\n%s", diff)
					}
				}

				if diff := cmp.Diff(tc.want.mg, tc.args.mg); diff != "" {
					t.Errorf("Create(...): -want, +got:\n%s", diff)
				}
			}
		})
	}
}
