package app

import (
	"context"
	"testing"

	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/pkg/errors"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/fake"
)

func TestFetchRoutes(t *testing.T) {
	errBoom := errors.New("boom")
	appGUID := "test-app-guid"
	port8080 := 8080

	type want struct {
		routes []v1alpha1.AppRouteObservation
		err    error
	}

	cases := map[string]struct {
		routeFetcher *fake.MockRouteFetcher
		want         want
	}{
		"SuccessfulWithRoutes": {
			routeFetcher: func() *fake.MockRouteFetcher {
				m := &fake.MockRouteFetcher{}
				m.On("ListForAppAll", appGUID).Return(
					[]*resource.Route{
						{
							URL:      "myapp.apps.example.com",
							Host:     "myapp",
							Path:     "",
							Protocol: "http",
							Port:     nil,
						},
						{
							URL:      "myapp.apps.example.com/admin",
							Host:     "myapp",
							Path:     "/admin",
							Protocol: "http",
							Port:     &port8080,
						},
					},
					nil,
				)
				return m
			}(),
			want: want{
				routes: []v1alpha1.AppRouteObservation{
					{
						URL:      "myapp.apps.example.com",
						Host:     "myapp",
						Path:     "",
						Protocol: "http",
						Port:     nil,
					},
					{
						URL:      "myapp.apps.example.com/admin",
						Host:     "myapp",
						Path:     "/admin",
						Protocol: "http",
						Port:     &port8080,
					},
				},
				err: nil,
			},
		},
		"EmptyRoutes": {
			routeFetcher: func() *fake.MockRouteFetcher {
				m := &fake.MockRouteFetcher{}
				m.On("ListForAppAll", appGUID).Return(
					[]*resource.Route{},
					nil,
				)
				return m
			}(),
			want: want{
				routes: []v1alpha1.AppRouteObservation{},
				err:    nil,
			},
		},
		"NilFetcher": {
			routeFetcher: nil,
			want: want{
				routes: nil,
				err:    nil,
			},
		},
		"ApiError": {
			routeFetcher: func() *fake.MockRouteFetcher {
				m := &fake.MockRouteFetcher{}
				m.On("ListForAppAll", appGUID).Return(
					([]*resource.Route)(nil),
					errBoom,
				)
				return m
			}(),
			want: want{
				routes: nil,
				err:    errBoom,
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			c := &Client{}
			if tc.routeFetcher != nil {
				c.RouteFetcher = tc.routeFetcher
			}

			routes, err := c.FetchRoutes(context.Background(), appGUID)

			if tc.want.err != nil {
				if err == nil {
					t.Fatalf("FetchRoutes() expected error, got nil")
				}
				if err.Error() != tc.want.err.Error() {
					t.Errorf("FetchRoutes() error = %v, want %v", err, tc.want.err)
				}
			} else if err != nil {
				t.Fatalf("FetchRoutes() unexpected error: %v", err)
			}

			if len(routes) != len(tc.want.routes) {
				t.Fatalf("FetchRoutes() got %d routes, want %d", len(routes), len(tc.want.routes))
			}

			for i, r := range routes {
				if r.URL != tc.want.routes[i].URL {
					t.Errorf("routes[%d].URL = %q, want %q", i, r.URL, tc.want.routes[i].URL)
				}
				if r.Host != tc.want.routes[i].Host {
					t.Errorf("routes[%d].Host = %q, want %q", i, r.Host, tc.want.routes[i].Host)
				}
				if r.Path != tc.want.routes[i].Path {
					t.Errorf("routes[%d].Path = %q, want %q", i, r.Path, tc.want.routes[i].Path)
				}
				if r.Protocol != tc.want.routes[i].Protocol {
					t.Errorf("routes[%d].Protocol = %q, want %q", i, r.Protocol, tc.want.routes[i].Protocol)
				}
				if tc.want.routes[i].Port != nil {
					if r.Port == nil {
						t.Errorf("routes[%d].Port = nil, want %d", i, *tc.want.routes[i].Port)
					} else if *r.Port != *tc.want.routes[i].Port {
						t.Errorf("routes[%d].Port = %d, want %d", i, *r.Port, *tc.want.routes[i].Port)
					}
				} else if r.Port != nil {
					t.Errorf("routes[%d].Port = %d, want nil", i, *r.Port)
				}
			}

			if tc.routeFetcher != nil {
				tc.routeFetcher.AssertExpectations(t)
			}
		})
	}
}
