package mta

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/fake"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/mta"
)

var (
	errBoom   = errors.New("boom")
	name      = "my-mta"
	spaceGUID = "a46808d1-d09a-4eef-add1-30872dec82f7"
	mtaGUID   = "2d8b0d04-d537-4e4e-8c6f-f09ca0e7f56f"
)

type modifier func(*v1alpha1.Mta)

func withExternalName(name string) modifier {
	return func(r *v1alpha1.Mta) {
		r.ObjectMeta.Annotations[meta.AnnotationKeyExternalName] = name
	}
}

func withSpace(space string) modifier {
	return func(r *v1alpha1.Mta) {
		r.Spec.ForProvider.Space = &space
	}
}

func withConditions(c ...xpv1.Condition) modifier {
	return func(i *v1alpha1.Mta) { i.Status.SetConditions(c...) }
}

func withStatus(mtaGUID string) modifier {
	o := v1alpha1.MtaObservation{}
	o.MtaId = &mtaGUID

	return func(r *v1alpha1.Mta) {
		r.Status.AtProvider = o
	}
}

func withUrl(image string) modifier {
	return func(r *v1alpha1.Mta) {
		r.Spec.ForProvider.File.URL = &image
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
					withExternalName(mtaGUID)),
				obs: managed.ExternalCreation{},
				err: nil,
			},
			service: func() *fake.MockMTA {
				m := &fake.MockMTA{}
				m.On("StartMtaOperation").Return(
					&fake.NewMta().SetMetadataID(mtaGUID).Mta,
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
