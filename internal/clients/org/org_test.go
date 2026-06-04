package org

import (
	"context"
	"testing"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"

	cfclient "github.com/cloudfoundry/go-cfclient/v3/client"
	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"k8s.io/utils/ptr"
)

var (
	testOrgName = "test-org"
	testOrgGUID = "2d8b0d04-d537-4e4e-8c6f-f09ca0e7f56f"
)

// mockClient implements the Client interface for testing
type mockClient struct {
	getFn    func(ctx context.Context, guid string) (*cfresource.Organization, error)
	singleFn func(ctx context.Context, opt *cfclient.OrganizationListOptions) (*cfresource.Organization, error)
	createFn func(ctx context.Context, opt *cfresource.OrganizationCreate) (*cfresource.Organization, error)
	deleteFn func(ctx context.Context, guid string) (string, error)
}

func (m *mockClient) Get(ctx context.Context, guid string) (*cfresource.Organization, error) {
	return m.getFn(ctx, guid)
}

func (m *mockClient) Single(ctx context.Context, opt *cfclient.OrganizationListOptions) (*cfresource.Organization, error) {
	return m.singleFn(ctx, opt)
}

func (m *mockClient) Create(ctx context.Context, opt *cfresource.OrganizationCreate) (*cfresource.Organization, error) {
	return m.createFn(ctx, opt)
}

func (m *mockClient) Delete(ctx context.Context, guid string) (string, error) {
	return m.deleteFn(ctx, guid)
}

func TestFindOrgBySpec(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		spec v1alpha1.OrgParameters
	}

	type want struct {
		org *cfresource.Organization
		err error
	}

	cases := map[string]struct {
		args args
		want want
		fn   func() *mockClient
	}{
		"Success": {
			args: args{
				spec: v1alpha1.OrgParameters{
					Name:      testOrgName,
					Suspended: ptr.To(false),
				},
			},
			want: want{
				org: &cfresource.Organization{
					Name: testOrgName,
					Resource: cfresource.Resource{
						GUID: testOrgGUID,
					},
				},
				err: nil,
			},
			fn: func() *mockClient {
				return &mockClient{
					singleFn: func(_ context.Context, _ *cfclient.OrganizationListOptions) (*cfresource.Organization, error) {
						return &cfresource.Organization{
							Name: testOrgName,
							Resource: cfresource.Resource{
								GUID: testOrgGUID,
							},
						}, nil
					},
				}
			},
		},
		"NotFound": {
			args: args{
				spec: v1alpha1.OrgParameters{
					Name: testOrgName,
				},
			},
			want: want{
				org: nil,
				err: nil,
			},
			fn: func() *mockClient {
				return &mockClient{
					singleFn: func(_ context.Context, _ *cfclient.OrganizationListOptions) (*cfresource.Organization, error) {
						return nil, nil
					},
				}
			},
		},
		"Error": {
			args: args{
				spec: v1alpha1.OrgParameters{
					Name: testOrgName,
				},
			},
			want: want{
				org: nil,
				err: errBoom,
			},
			fn: func() *mockClient {
				return &mockClient{
					singleFn: func(_ context.Context, _ *cfclient.OrganizationListOptions) (*cfresource.Organization, error) {
						return nil, errBoom
					},
				}
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			c := tc.fn()
			got, err := FindOrgBySpec(context.Background(), c, tc.args.spec)

			if tc.want.err != nil || err != nil {
				wantErr := ""
				gotErr := ""
				if tc.want.err != nil {
					wantErr = tc.want.err.Error()
				}
				if err != nil {
					gotErr = err.Error()
				}
				if diff := cmp.Diff(wantErr, gotErr); diff != "" {
					t.Errorf("FindOrgBySpec(...): want error string != got error string:\n%s", diff)
				}
			}
			if diff := cmp.Diff(tc.want.org, got); diff != "" {
				t.Errorf("FindOrgBySpec(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestGetOrgByGUID(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		guid string
	}

	type want struct {
		org *cfresource.Organization
		err error
	}

	cases := map[string]struct {
		args args
		want want
		fn   func() *mockClient
	}{
		"Success": {
			args: args{
				guid: testOrgGUID,
			},
			want: want{
				org: &cfresource.Organization{
					Name: testOrgName,
					Resource: cfresource.Resource{
						GUID: testOrgGUID,
					},
				},
				err: nil,
			},
			fn: func() *mockClient {
				return &mockClient{
					getFn: func(_ context.Context, guid string) (*cfresource.Organization, error) {
						return &cfresource.Organization{
							Name: testOrgName,
							Resource: cfresource.Resource{
								GUID: guid,
							},
						}, nil
					},
				}
			},
		},
		"Error": {
			args: args{
				guid: testOrgGUID,
			},
			want: want{
				org: nil,
				err: errBoom,
			},
			fn: func() *mockClient {
				return &mockClient{
					getFn: func(_ context.Context, _ string) (*cfresource.Organization, error) {
						return nil, errBoom
					},
				}
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			c := tc.fn()
			got, err := GetOrgByGUID(context.Background(), c, tc.args.guid)

			if tc.want.err != nil || err != nil {
				wantErr := ""
				gotErr := ""
				if tc.want.err != nil {
					wantErr = tc.want.err.Error()
				}
				if err != nil {
					gotErr = err.Error()
				}
				if diff := cmp.Diff(wantErr, gotErr); diff != "" {
					t.Errorf("GetOrgByGUID(...): want error string != got error string:\n%s", diff)
				}
			}
			if diff := cmp.Diff(tc.want.org, got); diff != "" {
				t.Errorf("GetOrgByGUID(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestLateInitialize(t *testing.T) {
	labels := map[string]*string{"team": ptr.To("platform")}
	annotations := map[string]*string{"note": ptr.To("managed-elsewhere")}

	spec := &v1alpha1.OrgParameters{}
	from := &cfresource.Organization{
		Name:      testOrgName,
		Suspended: true,
		Metadata: &cfresource.Metadata{
			Labels: map[string]*string{
				"crossplane-kind": ptr.To("organization.cloudfoundry.crossplane.io"),
				"crossplane-name": ptr.To("my-org"),
				"team":            labels["team"],
			},
			Annotations: annotations,
		},
	}

	LateInitialize(spec, from)

	if spec.Name != testOrgName {
		t.Fatalf("LateInitialize(...): expected Name %q, got %q", testOrgName, spec.Name)
	}
	if spec.Suspended == nil || !*spec.Suspended {
		t.Fatalf("LateInitialize(...): expected Suspended to be true, got %#v", spec.Suspended)
	}
	if spec.Labels != nil {
		t.Fatalf("LateInitialize(...): expected Labels to remain nil, got %#v", spec.Labels)
	}
	if spec.Annotations != nil {
		t.Fatalf("LateInitialize(...): expected Annotations to remain nil, got %#v", spec.Annotations)
	}
}

// Ensure mockClient satisfies the Client interface
var _ Client = &mockClient{}
