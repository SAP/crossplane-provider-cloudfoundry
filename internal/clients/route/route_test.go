package route

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/cloudfoundry/go-cfclient/v3/client"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/fake"
)

var (
	url = "test-url"

	guid       = "33fd5b0b-4f3b-4b1b-8b3d-3b5f7b4b3b4b"
	spaceGUID  = "11fd5b0b-4f3b-4b1b-8b3d-3b5f7b4b3b4b"
	domainGUID = "22fd5b0b-4f3b-4b1b-8b3d-3b5f7b4b3b4b"
	timezero   = "0001-01-01T00:00:00Z"

	fakeForProvider = v1alpha1.RouteParameters{
		SpaceReference:  v1alpha1.SpaceReference{Space: &spaceGUID},
		DomainReference: v1alpha1.DomainReference{Domain: &domainGUID},
	}

	emptyForProvider = v1alpha1.RouteParameters{}

	fakeObservation = &v1alpha1.RouteObservation{
		Resource: v1alpha1.Resource{
			GUID:      guid,
			CreatedAt: &timezero,
			UpdatedAt: &timezero,
		},
		URL: &url,
	}

	errBoom             = errors.New("boom")
	errNoResultReturned = client.ErrNoResultsReturned
)

func TestFindRouteBySpec(t *testing.T) {
	type service func() *fake.MockRoute
	type args struct {
		forProvider v1alpha1.RouteParameters
	}

	type want struct {
		observation *v1alpha1.RouteObservation
		exists      bool
		err         error
	}

	cases := map[string]struct {
		args    args
		want    want
		service service
	}{
		"Found": {
			args: args{
				forProvider: fakeForProvider,
			},
			want: want{
				observation: fakeObservation,
				exists:      true,
				err:         nil,
			},
			service: func() *fake.MockRoute {
				m := &fake.MockRoute{}
				m.On("Single").Return(
					fake.FakeRoute(guid, url),
					nil,
				)
				return m
			},
		},
		"NotFound": {
			args: args{
				forProvider: fakeForProvider,
			},
			want: want{
				observation: nil,
				exists:      false,
				err:         nil,
			},
			service: func() *fake.MockRoute {
				m := &fake.MockRoute{}
				m.On("Single").Return(
					fake.RouteNil,
					errNoResultReturned,
				)
				return m
			},
		},
		"Error": {
			args: args{
				forProvider: fakeForProvider,
			},
			want: want{
				observation: nil,
				exists:      false,
				err:         errBoom,
			},
			service: func() *fake.MockRoute {
				m := &fake.MockRoute{}
				m.On("Single").Return(
					fake.RouteNil,
					errBoom,
				)
				return m
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			c := &Client{
				Route: tc.service(),
			}

			obs, exists, err := c.FindRouteBySpec(context.Background(), tc.args.forProvider)

			if tc.want.err != nil && err != nil {
				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("FindRouteBySpec(...): want error string != got error string:\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(tc.want.err, err); diff != "" {
					t.Errorf("FindRouteBySpec(...): want error != got error:\n%s", diff)
				}
			}
			if diff := cmp.Diff(tc.want.exists, exists); diff != "" {
				t.Errorf("FindRouteBySpec(...): -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.observation, obs); diff != "" {
				t.Errorf("FindRouteBySpec(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestGetRouteByGUID(t *testing.T) {
	type service func() *fake.MockRoute
	type args struct {
		guid string
	}

	type want struct {
		observation *v1alpha1.RouteObservation
		exists      bool
		err         error
	}

	cases := map[string]struct {
		args    args
		want    want
		service service
	}{
		"Found": {
			args: args{
				guid: guid,
			},
			want: want{
				observation: fakeObservation,
				exists:      true,
				err:         nil,
			},
			service: func() *fake.MockRoute {
				m := &fake.MockRoute{}
				m.On("Get", guid).Return(
					fake.FakeRoute(guid, url),
					nil,
				)
				return m
			},
		},
		"NotFound": {
			args: args{
				guid: guid,
			},
			want: want{
				observation: nil,
				exists:      false,
				err:         nil,
			},
			service: func() *fake.MockRoute {
				m := &fake.MockRoute{}
				m.On("Get", guid).Return(
					fake.RouteNil,
					errNoResultReturned,
				)
				return m
			},
		},
		"Error": {
			args: args{
				guid: guid,
			},
			want: want{
				observation: nil,
				exists:      false,
				err:         errBoom,
			},
			service: func() *fake.MockRoute {
				m := &fake.MockRoute{}
				m.On("Get", guid).Return(
					fake.RouteNil,
					errBoom,
				)
				return m
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			c := &Client{
				Route: tc.service(),
			}

			obs, exists, err := c.GetRouteByGUID(context.Background(), tc.args.guid)

			if tc.want.err != nil && err != nil {
				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("GetRouteByGUID(...): want error string != got error string:\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(tc.want.err, err); diff != "" {
					t.Errorf("GetRouteByGUID(...): want error != got error:\n%s", diff)
				}
			}
			if diff := cmp.Diff(tc.want.exists, exists); diff != "" {
				t.Errorf("GetRouteByGUID(...): -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.observation, obs); diff != "" {
				t.Errorf("GetRouteByGUID(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestCreate(t *testing.T) {
	type service func() *fake.MockRoute
	type args struct {
		forProvider v1alpha1.RouteParameters
	}

	type want struct {
		guid string
		err  error
	}

	cases := map[string]struct {
		args    args
		want    want
		service service
	}{
		"should error when API errors": {
			args: args{
				forProvider: fakeForProvider,
			},
			want: want{
				guid: "",
				err:  errBoom,
			},
			service: func() *fake.MockRoute {
				m := &fake.MockRoute{}
				m.On("Create").Return(
					fake.FakeRoute(guid, url),
					errBoom,
				)
				return m
			},
		},
		"should error when space or domain is missing": {
			args: args{
				forProvider: emptyForProvider,
			},
			want: want{
				guid: "",
				err:  errors.New("space and domain are required"),
			},
			service: func() *fake.MockRoute {
				m := &fake.MockRoute{}
				return m
			},
		},

		"should create": {
			args: args{
				forProvider: fakeForProvider,
			},
			want: want{
				guid: guid,
				err:  nil,
			},
			service: func() *fake.MockRoute {
				m := &fake.MockRoute{}
				m.On("Create").Return(
					fake.FakeRoute(guid, url),
					nil,
				)
				return m
			},
		},
	}
	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			t.Logf("Testing: %s", t.Name())
			c := &Client{
				Route: tc.service(),
			}

			id, err := c.Create(context.Background(), tc.args.forProvider)

			if tc.want.err != nil && err != nil {
				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("Create(...): want error string != got error string:\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(tc.want.err, err); diff != "" {
					t.Errorf("Create(...): want error != got error:\n%s", diff)
				}
			}
			if diff := cmp.Diff(tc.want.guid, id); diff != "" {
				t.Errorf("Create(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	type service func() *fake.MockRoute
	type args struct {
		guid string
	}

	type want struct {
		jobGUID string
		err     error
	}

	cases := map[string]struct {
		args    args
		want    want
		service service
	}{
		"Successful": {
			args: args{
				guid: guid,
			},
			want: want{
				jobGUID: "job-guid-123",
				err:     nil,
			},
			service: func() *fake.MockRoute {
				m := &fake.MockRoute{}
				m.On("Delete").Return(
					"job-guid-123",
					nil,
				)
				return m
			},
		},
		"NotFound": {
			args: args{
				guid: guid,
			},
			want: want{
				jobGUID: "",
				err:     nil,
			},
			service: func() *fake.MockRoute {
				m := &fake.MockRoute{}
				m.On("Delete").Return(
					"",
					errNoResultReturned,
				)
				return m
			},
		},
		"InvalidGUID": {
			args: args{
				guid: "not-valid",
			},
			want: want{
				jobGUID: "",
				err:     errors.New("invalid Route GUID"),
			},
			service: func() *fake.MockRoute {
				m := &fake.MockRoute{}
				return m
			},
		},
		"Error": {
			args: args{
				guid: guid,
			},
			want: want{
				jobGUID: "",
				err:     errBoom,
			},
			service: func() *fake.MockRoute {
				m := &fake.MockRoute{}
				m.On("Delete").Return(
					"",
					errBoom,
				)
				return m
			},
		},
	}
	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			t.Logf("Testing: %s", t.Name())
			c := &Client{
				Route: tc.service(),
			}

			jobGUID, err := c.Delete(context.Background(), tc.args.guid)

			if tc.want.err != nil && err != nil {
				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("Delete(...): want error string != got error string:\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(tc.want.err, err); diff != "" {
					t.Errorf("Delete(...): want error != got error:\n%s", diff)
				}
			}
			if diff := cmp.Diff(tc.want.jobGUID, jobGUID); diff != "" {
				t.Errorf("Delete(...): -want, +got:\n%s", diff)
			}
		})
	}
}
