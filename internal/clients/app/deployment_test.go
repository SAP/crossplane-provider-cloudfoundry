package app

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/fake"
)

func TestPendingRestart(t *testing.T) {
	cases := map[string]struct {
		annotations map[string]string
		lastHandled string
		wantValue   string
		wantPending bool
	}{
		"NilAnnotations":    {annotations: nil},
		"AnnotationAbsent":  {annotations: map[string]string{"other": "x"}},
		"EmptyValue":        {annotations: map[string]string{"app.cloudfoundry.crossplane.io/restart": ""}},
		"NewValue":          {annotations: map[string]string{"app.cloudfoundry.crossplane.io/restart": "a"}, wantValue: "a", wantPending: true},
		"AlreadyHandled":    {annotations: map[string]string{"app.cloudfoundry.crossplane.io/restart": "a"}, lastHandled: "a"},
		"ReturnToOldValue":  {annotations: map[string]string{"app.cloudfoundry.crossplane.io/restart": "a"}, lastHandled: "b", wantValue: "a", wantPending: true},
		"WhitespaceIsValue": {annotations: map[string]string{"app.cloudfoundry.crossplane.io/restart": " "}, wantValue: " ", wantPending: true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cr := &v1alpha1.App{ObjectMeta: metav1.ObjectMeta{Annotations: tc.annotations}}
			cr.Status.AtProvider.LastHandledRestart = tc.lastHandled

			value, pending := PendingRestart(cr)
			if value != tc.wantValue || pending != tc.wantPending {
				t.Errorf("PendingRestart() = (%q, %t), want (%q, %t)", value, pending, tc.wantValue, tc.wantPending)
			}
		})
	}
}

func TestRestartStamp(t *testing.T) {
	// sha256("a"), computed independently.
	if got := RestartStamp("a"); got != "ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb" {
		t.Errorf("RestartStamp(a) = %q", got)
	}
	long := RestartStamp(strings.Repeat("ä", 10000))
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(long) {
		t.Errorf("RestartStamp(long) = %q, want 64 lowercase hex chars", long)
	}
}

func TestIsRestartDeployment(t *testing.T) {
	stamped := func(value string) *resource.Deployment {
		return &resource.Deployment{Metadata: &resource.Metadata{Annotations: map[string]*string{
			"crossplane-restart-sha256": ptr.To(value),
		}}}
	}
	cases := map[string]struct {
		d    *resource.Deployment
		want bool
	}{
		"NilMetadata":   {d: &resource.Deployment{}, want: false},
		"NilStampValue": {d: &resource.Deployment{Metadata: &resource.Metadata{Annotations: map[string]*string{"crossplane-restart-sha256": nil}}}, want: false},
		"OtherValue":    {d: stamped("3e23e8160039594a33894f6564e1b1348bbd7a0088d42c4acb73eeaed59c009d"), want: false}, // sha256("b")
		"SameValue":     {d: stamped("ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb"), want: true},  // sha256("a")
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := IsRestartDeployment(tc.d, "a"); got != tc.want {
				t.Errorf("IsRestartDeployment() = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestLatestDeployment(t *testing.T) {
	latest := &resource.Deployment{Resource: resource.Resource{GUID: "d1"}}
	errBoom := errors.New("boom")
	isLatestQuery := mock.MatchedBy(func(o *client.DeploymentListOptions) bool {
		return cmp.Equal(o.AppGUIDs.Values, []string{testAppGUID}) && len(o.StatusValues.Values) == 0 &&
			o.OrderBy == "-created_at" && o.PerPage == 1
	})
	cases := map[string]struct {
		result  []*resource.Deployment
		listErr error
		want    *resource.Deployment
		wantErr error
	}{
		"None":    {result: []*resource.Deployment{}},
		"One":     {result: []*resource.Deployment{latest}, want: latest},
		"ListErr": {result: nil, listErr: errBoom, wantErr: errBoom},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			m := &fake.MockDeployment{}
			m.On("List", isLatestQuery).Return(tc.result, tc.listErr)
			c := &Client{Deployments: m}

			got, err := c.LatestDeployment(context.Background(), testAppGUID)
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("LatestDeployment() error = %v, want %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("LatestDeployment() = %v, want %v", got, tc.want)
			}
			m.AssertExpectations(t)
		})
	}
}

func TestStartRollingRestart(t *testing.T) {
	created := &resource.Deployment{Resource: resource.Resource{GUID: "d1"}}
	m := &fake.MockDeployment{}
	m.On("Create", mock.Anything).Return(created, nil)
	c := &Client{Deployments: m}

	got, err := c.StartRollingRestart(context.Background(), testAppGUID, "a")
	if err != nil || got != created {
		t.Fatalf("StartRollingRestart() = (%v, %v), want (%v, nil)", got, err, created)
	}

	sent := m.Calls[0].Arguments.Get(0).(*resource.DeploymentCreate)
	want := &resource.DeploymentCreate{
		Relationships: resource.AppRelationship{App: resource.ToOneRelationship{Data: &resource.Relationship{GUID: testAppGUID}}},
		Strategy:      "rolling",
		Metadata: &resource.Metadata{Annotations: map[string]*string{
			"crossplane-restart-sha256": ptr.To("ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb"),
		}},
	}
	if diff := cmp.Diff(want, sent); diff != "" {
		t.Errorf("DeploymentCreate: -want, +got:\n%s", diff)
	}
}

func TestStartRollingRestartError(t *testing.T) {
	errBoom := errors.New("boom")
	m := &fake.MockDeployment{}
	m.On("Create", mock.Anything).Return((*resource.Deployment)(nil), errBoom)
	c := &Client{Deployments: m}

	if _, err := c.StartRollingRestart(context.Background(), testAppGUID, "a"); !errors.Is(err, errBoom) {
		t.Errorf("StartRollingRestart() error = %v, want %v", err, errBoom)
	}
}
