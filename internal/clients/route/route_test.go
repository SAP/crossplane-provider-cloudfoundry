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
	nilObservation *v1alpha1.RouteObservation

	errBoom                        = errors.New("boom")
	errNoResultReturned            = client.ErrNoResultsReturned
	errExactlyOneResultNotReturned = client.ErrExactlyOneResultNotReturned
)

func TestGetByIDOrName(t *testing.T) {
	type service func() *fake.MockRoute
	type args struct {
		guid        string
		forProvider v1alpha1.RouteParameters
	}

	type want struct {
		atProvider *v1alpha1.RouteObservation
		err        error
	}

	cases := map[string]struct {
		args    args
		want    want
		service service
	}{
		"should error when API errors": {
			args: args{
				guid:        guid,
				forProvider: fakeForProvider,
			},
			want: want{
				atProvider: nilObservation,
				err:        errBoom,
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
		"should return nil and ignore error when no result returned": {
			args: args{
				guid:        guid,
				forProvider: fakeForProvider,
			},
			want: want{
				atProvider: nilObservation,
				err:        nil,
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
		"should return error when exactly one result not returned": {
			args: args{
				guid:        "not-valid",
				forProvider: fakeForProvider,
			},
			want: want{
				atProvider: nilObservation,
				err:        nil,
			},
			service: func() *fake.MockRoute {
				m := &fake.MockRoute{}
				m.On("Single").Return(
					fake.RouteNil,
					errExactlyOneResultNotReturned,
				)
				return m
			},
		},

		"should get by id": {
			args: args{
				guid:        guid,
				forProvider: fakeForProvider,
			},
			want: want{
				atProvider: fakeObservation,
				err:        nil,
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
		"should get by spec": {
			args: args{
				guid:        "not-valid",
				forProvider: fakeForProvider,
			},
			want: want{
				atProvider: fakeObservation,
				err:        nil,
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
	}
	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			t.Logf("Testing: %s", t.Name())
			c := &Client{
				Route: tc.service(),
			}

			obs, err := c.GetByIDOrSpec(context.Background(), tc.args.guid, tc.args.forProvider)

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
			if diff := cmp.Diff(tc.want.atProvider, obs); diff != "" {
				t.Errorf("Observe(...): -want, +got:\n%s", diff)
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
				err:  errors.New("Space and Domain are required"),
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
				// the case where our mock server returns error.
				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("Observe(...): want error string != got error string:\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(tc.want.err, err); diff != "" {
					t.Errorf("Observe(...): want error != got error:\n%s", diff)
				}
			}
			if diff := cmp.Diff(tc.want.guid, id); diff != "" {
				t.Errorf("Observe(...): -want, +got:\n%s", diff)
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
		err error
	}

	cases := map[string]struct {
		args    args
		want    want
		service service
	}{
		"should error when API errors": {
			args: args{
				guid: guid,
			},
			want: want{
				err: errBoom,
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
		"should error when guid is invalid": {
			args: args{
				guid: "not-valid",
			},
			want: want{
				err: errors.New("invalid Route GUID"),
			},
			service: func() *fake.MockRoute {
				m := &fake.MockRoute{}
				return m
			},
		},

		"should delete": {
			args: args{
				guid: guid,
			},
			want: want{
				err: nil,
			},
			service: func() *fake.MockRoute {
				m := &fake.MockRoute{}
				m.On("Delete").Return(
					"",
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

			err := c.Delete(context.Background(), tc.args.guid)
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
