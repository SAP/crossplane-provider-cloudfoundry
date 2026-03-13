package serviceinstance

import (
	"context"
	"fmt"
	"testing"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/fake"
)

var (
	guid           = "2d8b0d04-d537-4e4e-8c6f-f09ca0e7f56f"
	offering       = "test-offering"
	plan           = "test-plan"
	validateErrMsg = fmt.Sprintf("error executing GET request for %s: cfclient error (CF-ResourceNotFound|10010): Service plan not found", guid)

	errValidate                    = errors.New(validateErrMsg)
	errExactlyOneResultNotReturned = client.ErrExactlyOneResultNotReturned
)

type modifier func(*v1alpha1.ServiceInstance)

func withServicePlan(servicePlan v1alpha1.ServicePlanParameters) modifier {
	return func(r *v1alpha1.ServiceInstance) {
		r.Spec.ForProvider.ServicePlan = &servicePlan
	}
}

func serviceInstance(m ...modifier) *v1alpha1.ServiceInstance {
	r := &v1alpha1.ServiceInstance{}
	for _, rm := range m {
		rm(r)
	}
	return r
}

func TestResolveServicePlan(t *testing.T) {
	type service func() ServicePlan
	type args struct {
		ctx context.Context
		cr  *v1alpha1.ServiceInstance
	}
	type want struct {
		err error
	}

	cases := map[string]struct {
		args    args
		want    want
		service service
		kube    k8s.Client
	}{
		"None": {
			args: args{
				ctx: context.Background(),
				cr:  serviceInstance(withServicePlan(v1alpha1.ServicePlanParameters{})),
			},
			want: want{
				err: errors.New(errMissingServicePlan),
			},
			service: func() ServicePlan {
				m := &fake.MockServicePlan{}
				return m
			},
		},
		"OnlyId": {
			args: args{
				ctx: context.Background(),
				cr:  serviceInstance(withServicePlan(v1alpha1.ServicePlanParameters{ID: &guid})),
			},
			want: want{
				err: nil,
			},
			service: func() ServicePlan {
				m := &fake.MockServicePlan{}
				m.On("Get", guid).Return(&resource.ServicePlan{
					Resource: resource.Resource{
						GUID: guid,
					},
				}, nil)
				return m
			},
		},
		"OfferingAndPlan": {
			args: args{
				ctx: context.Background(),
				cr:  serviceInstance(withServicePlan(v1alpha1.ServicePlanParameters{Offering: &offering, Plan: &plan})),
			},
			want: want{
				err: nil,
			},
			service: func() ServicePlan {
				m := &fake.MockServicePlan{}
				m.On("Single").Return(&resource.ServicePlan{
					Resource: resource.Resource{
						GUID: guid,
					},
				}, nil)
				return m
			},
		},
		"OnlyOffering": {
			args: args{
				ctx: context.Background(),
				cr:  serviceInstance(withServicePlan(v1alpha1.ServicePlanParameters{Offering: &offering})),
			},
			want: want{
				err: errors.New(errMissingServicePlan),
			},
			service: func() ServicePlan {
				m := &fake.MockServicePlan{}
				m.On("Single").Return(&resource.ServicePlan{}, errors.New("failed"))
				return m
			},
		},
		"OnlyPlan": {
			args: args{
				ctx: context.Background(),
				cr:  serviceInstance(withServicePlan(v1alpha1.ServicePlanParameters{Plan: &plan})),
			},
			want: want{
				err: errors.New(errMissingServicePlan),
			},
			service: func() ServicePlan {
				m := &fake.MockServicePlan{}
				m.On("Single").Return(&resource.ServicePlan{}, errors.New("failed"))
				return m
			},
		},
		"All": {
			args: args{
				ctx: context.Background(),
				cr:  serviceInstance(withServicePlan(v1alpha1.ServicePlanParameters{ID: &guid, Plan: &plan, Offering: &offering})),
			},
			want: want{
				err: nil,
			},
			service: func() ServicePlan {
				m := &fake.MockServicePlan{}
				m.On("Single").Return(&resource.ServicePlan{
					Resource: resource.Resource{
						GUID: guid,
					},
				}, nil)
				return m
			},
		},
		"ErrorResolvePlanID": {
			args: args{
				ctx: context.Background(),
				cr:  serviceInstance(withServicePlan(v1alpha1.ServicePlanParameters{Plan: &plan, Offering: &offering})),
			},
			want: want{
				err: errors.Wrapf(errExactlyOneResultNotReturned, "cannot initialize service plan using serviceName/servicePlanName: %s:%s", offering, plan),
			},
			service: func() ServicePlan {
				m := &fake.MockServicePlan{}
				m.On("Single").Return(&resource.ServicePlan{}, errExactlyOneResultNotReturned)
				return m
			},
		},
		"ErrorValidatePlanID": {
			args: args{
				ctx: context.Background(),
				cr:  serviceInstance(withServicePlan(v1alpha1.ServicePlanParameters{ID: &guid})),
			},
			want: want{
				err: errors.Wrapf(errValidate, "cannot initialize service plan using ID: %s", guid),
			},
			service: func() ServicePlan {
				m := &fake.MockServicePlan{}
				m.On("Get", guid).Return(&resource.ServicePlan{}, errValidate)
				return m
			},
		},
	}
	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			c := &Client{
				ServicePlanResolver: tc.service(),
			}
			kube := &test.MockClient{
				MockUpdate: test.NewMockUpdateFn(nil),
			}
			err := c.ResolveServicePlan(tc.args.ctx, kube, tc.args.cr)

			switch {
			case tc.want.err == nil && err != nil:
				t.Fatalf("ResolveServicePlan(...): unexpected error: %v", err)

			case tc.want.err != nil && err == nil:
				t.Fatalf("ResolveServicePlan(...): expected error %v but got nil", tc.want.err)

			case tc.want.err != nil && err != nil:
				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("ResolveServicePlan(...): -want, +got:\n%s", diff)
				}
			}
		})
	}
}
