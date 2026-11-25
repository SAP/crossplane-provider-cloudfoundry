package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/app"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/fake"
)

const (
	deploymentGUID = "5d1f6b2c-3a4e-4f6b-9c1d-2e3f4a5b6c7d"
	restartKey     = "app.cloudfoundry.crossplane.io/restart"
	// sha256("a"), computed independently.
	stampA = "ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb"
)

var deploymentCreatedAt = time.Date(2026, 11, 17, 10, 0, 0, 0, time.UTC)

type fakeRecorder struct {
	events []event.Event
}

func (r *fakeRecorder) Event(_ runtime.Object, e event.Event) { r.events = append(r.events, e) }

func (r *fakeRecorder) WithAnnotations(...string) event.Recorder { return r }

// kinds returns "Type/Reason" for every recorded event.
func (r *fakeRecorder) kinds() []string {
	out := make([]string, 0, len(r.events))
	for _, e := range r.events {
		out = append(out, string(e.Type)+"/"+string(e.Reason))
	}
	return out
}

func withRestartAnnotation(v string) modifier {
	return func(r *v1alpha1.App) { r.Annotations[restartKey] = v }
}

func withLastHandledRestart(v string) modifier {
	return func(r *v1alpha1.App) { r.Status.AtProvider.LastHandledRestart = v }
}

func withDeployment(d *v1alpha1.AppDeploymentObservation) modifier {
	return func(r *v1alpha1.App) { r.Status.AtProvider.Deployment = d }
}

// trackedDeployment is the status record of deploymentGUID in the given CF state.
func trackedDeployment(state, reason, restart string) *v1alpha1.AppDeploymentObservation {
	return &v1alpha1.AppDeploymentObservation{
		GUID:      deploymentGUID,
		Strategy:  "rolling",
		State:     state,
		Reason:    reason,
		CreatedAt: ptr.To("2026-11-17T10:00:00Z"),
		Restart:   restart,
	}
}

// cfDeployment is deploymentGUID as returned by the CF API.
func cfDeployment(state, reason string, annotations map[string]*string) *cfresource.Deployment {
	return &cfresource.Deployment{
		Resource: cfresource.Resource{GUID: deploymentGUID, CreatedAt: deploymentCreatedAt, UpdatedAt: deploymentCreatedAt},
		Strategy: "rolling",
		Status:   cfresource.DeploymentStatus{Value: state, Reason: reason},
		Metadata: &cfresource.Metadata{Annotations: annotations},
	}
}

// observedCFApp is the CF app the "Successful" TestObserve case considers up to date.
func observedCFApp(state string) *cfresource.App {
	return &fake.NewApp("docker").SetName(name).SetGUID(guid).SetAnnotations(map[string]*string{
		"crossplane-kind": ptr.To("app.cloudfoundry.crossplane.io"),
		"crossplane-name": ptr.To("my-app"),
	}).SetState(state).App
}

func restartApp(m ...modifier) *v1alpha1.App {
	return newApp("docker", append([]modifier{withExternalName(guid), withSpace(spaceGUID), withDefaultMetadata()}, m...)...)
}

// specDrift adds a label CF does not have, so IsUpToDate reports false.
var specDrift = withLabels(map[string]*string{"x": ptr.To("y")})

func TestObserveDeployment(t *testing.T) {
	// A rollout that makes no progress: CF reports when instances were last
	// healthy and why scaling failed.
	stalled := cfDeployment("ACTIVE", "DEPLOYING", nil)
	stalled.Status.Details = &cfresource.DeploymentStatusDetails{
		LastSuccessfulHealthcheck: ptr.To(deploymentCreatedAt.Add(time.Minute)),
		Error:                     "memory quota_exceeded",
	}
	stalledObs := trackedDeployment("ACTIVE", "DEPLOYING", "a")
	stalledObs.LastSuccessfulHealthcheck = ptr.To("2026-11-17T10:01:00Z")
	stalledObs.Error = "memory quota_exceeded"

	cases := map[string]struct {
		mg              *v1alpha1.App
		appGetErr       bool
		deployments     func(m *fake.MockDeployment)
		wantObs         managed.ExternalObservation
		statusErr       error
		wantErr         string
		wantDeployment  *v1alpha1.AppDeploymentObservation
		wantLastHandled string
		wantEvents      []string
		wantMessage     []string
		wantWrites      int
	}{
		"PreservesTrackingFields": {
			mg:              restartApp(withRestartAnnotation("a"), withLastHandledRestart("a"), withDeployment(trackedDeployment("FINALIZED", "DEPLOYED", "a"))),
			wantObs:         managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
			wantDeployment:  trackedDeployment("FINALIZED", "DEPLOYED", "a"),
			wantLastHandled: "a",
			wantEvents:      []string{},
		},
		"ActiveDeploymentHoldsDrift": {
			mg: restartApp(specDrift, withRestartAnnotation("b"), withLastHandledRestart("a"), withDeployment(trackedDeployment("ACTIVE", "DEPLOYING", "a"))),
			deployments: func(m *fake.MockDeployment) {
				m.On("Get", deploymentGUID).Return(cfDeployment("ACTIVE", "DEPLOYING", nil), nil)
			},
			wantObs:         managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
			wantDeployment:  trackedDeployment("ACTIVE", "DEPLOYING", "a"),
			wantLastHandled: "a",
			wantEvents:      []string{},
		},
		"ActiveRefreshesProgress": {
			mg: restartApp(withRestartAnnotation("a"), withLastHandledRestart("a"), withDeployment(trackedDeployment("ACTIVE", "DEPLOYING", "a"))),
			deployments: func(m *fake.MockDeployment) {
				m.On("Get", deploymentGUID).Return(stalled, nil)
			},
			wantObs:         managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
			wantDeployment:  stalledObs,
			wantLastHandled: "a",
			wantEvents:      []string{},
		},
		"TransitionDeployed": {
			mg: restartApp(withRestartAnnotation("a"), withLastHandledRestart("a"), withDeployment(trackedDeployment("ACTIVE", "DEPLOYING", "a"))),
			deployments: func(m *fake.MockDeployment) {
				m.On("Get", deploymentGUID).Return(cfDeployment("FINALIZED", "DEPLOYED", nil), nil)
			},
			wantObs:         managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
			wantDeployment:  trackedDeployment("FINALIZED", "DEPLOYED", "a"),
			wantLastHandled: "a",
			wantEvents:      []string{"Normal/DeploymentSucceeded"},
			wantWrites:      1,
		},
		"TransitionCanceled": {
			mg: restartApp(withRestartAnnotation("a"), withLastHandledRestart("a"), withDeployment(trackedDeployment("ACTIVE", "DEPLOYING", "a"))),
			deployments: func(m *fake.MockDeployment) {
				m.On("Get", deploymentGUID).Return(cfDeployment("FINALIZED", "CANCELED", nil), nil)
			},
			wantObs:         managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
			wantDeployment:  trackedDeployment("FINALIZED", "CANCELED", "a"),
			wantLastHandled: "a",
			wantEvents:      []string{"Warning/DeploymentFailed"},
			wantMessage:     []string{"CANCELED", deploymentGUID},
			wantWrites:      1,
		},
		"TransitionSuperseded": {
			mg: restartApp(withRestartAnnotation("a"), withLastHandledRestart("a"), withDeployment(trackedDeployment("ACTIVE", "DEPLOYING", "a"))),
			deployments: func(m *fake.MockDeployment) {
				m.On("Get", deploymentGUID).Return(cfDeployment("FINALIZED", "SUPERSEDED", nil), nil)
			},
			wantObs:         managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
			wantDeployment:  trackedDeployment("FINALIZED", "SUPERSEDED", "a"),
			wantLastHandled: "a",
			wantEvents:      []string{"Warning/DeploymentFailed"},
			wantMessage:     []string{"SUPERSEDED", deploymentGUID},
			wantWrites:      1,
		},
		"TransitionOpensPendingRestart": {
			mg: restartApp(withRestartAnnotation("b"), withLastHandledRestart("a"), withDeployment(trackedDeployment("ACTIVE", "DEPLOYING", "a"))),
			deployments: func(m *fake.MockDeployment) {
				m.On("Get", deploymentGUID).Return(cfDeployment("FINALIZED", "DEPLOYED", nil), nil)
			},
			wantObs:         managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: false},
			wantDeployment:  trackedDeployment("FINALIZED", "DEPLOYED", "a"),
			wantLastHandled: "a",
			wantEvents:      []string{"Normal/DeploymentSucceeded"},
			wantWrites:      1,
		},
		"NotFound": {
			mg: restartApp(withRestartAnnotation("a"), withLastHandledRestart("a"), withDeployment(trackedDeployment("ACTIVE", "DEPLOYING", "a"))),
			deployments: func(m *fake.MockDeployment) {
				m.On("Get", deploymentGUID).Return((*cfresource.Deployment)(nil),
					cfresource.CloudFoundryError{Code: 10010, Title: "CF-ResourceNotFound", Detail: "Deployment not found"})
			},
			wantObs:         managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
			wantDeployment:  trackedDeployment("FINALIZED", "NOT_FOUND", "a"),
			wantLastHandled: "a",
			wantEvents:      []string{"Warning/DeploymentFailed"},
			wantMessage:     []string{"NOT_FOUND", deploymentGUID},
			wantWrites:      1,
		},
		"FinalStatusWriteFails": {
			// Undone: the next poll saves and reports the outcome.
			mg: restartApp(withRestartAnnotation("a"), withLastHandledRestart("a"), withDeployment(trackedDeployment("ACTIVE", "DEPLOYING", "a"))),
			deployments: func(m *fake.MockDeployment) {
				m.On("Get", deploymentGUID).Return(cfDeployment("FINALIZED", "DEPLOYED", nil), nil)
			},
			statusErr:       errBoom,
			wantErr:         "boom",
			wantDeployment:  trackedDeployment("ACTIVE", "DEPLOYING", "a"),
			wantLastHandled: "a",
			wantEvents:      []string{},
			wantWrites:      1,
		},
		"NotFoundStatusWriteFails": {
			mg: restartApp(withRestartAnnotation("a"), withLastHandledRestart("a"), withDeployment(trackedDeployment("ACTIVE", "DEPLOYING", "a"))),
			deployments: func(m *fake.MockDeployment) {
				m.On("Get", deploymentGUID).Return((*cfresource.Deployment)(nil),
					cfresource.CloudFoundryError{Code: 10010, Title: "CF-ResourceNotFound", Detail: "Deployment not found"})
			},
			statusErr:       errBoom,
			wantErr:         "boom",
			wantDeployment:  trackedDeployment("ACTIVE", "DEPLOYING", "a"),
			wantLastHandled: "a",
			wantEvents:      []string{},
			wantWrites:      1,
		},
		"GetError": {
			mg: restartApp(withRestartAnnotation("a"), withLastHandledRestart("a"), withDeployment(trackedDeployment("ACTIVE", "DEPLOYING", "a"))),
			deployments: func(m *fake.MockDeployment) {
				m.On("Get", deploymentGUID).Return((*cfresource.Deployment)(nil), errBoom)
			},
			wantErr:         "boom",
			wantDeployment:  trackedDeployment("ACTIVE", "DEPLOYING", "a"),
			wantLastHandled: "a",
			wantEvents:      []string{},
		},
		"PendingRestartNotUpToDate": {
			mg:         restartApp(withRestartAnnotation("a")),
			wantObs:    managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: false},
			wantEvents: []string{},
		},
		"FinalTrackedDeploymentNotPolled": {
			mg:              restartApp(withLastHandledRestart("a"), withDeployment(trackedDeployment("FINALIZED", "DEPLOYED", "a"))),
			wantObs:         managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
			wantDeployment:  trackedDeployment("FINALIZED", "DEPLOYED", "a"),
			wantLastHandled: "a",
			wantEvents:      []string{},
		},
		"AppGoneWhileTracking": {
			mg:              restartApp(withRestartAnnotation("a"), withLastHandledRestart("a"), withDeployment(trackedDeployment("ACTIVE", "DEPLOYING", "a"))),
			appGetErr:       true,
			wantObs:         managed.ExternalObservation{ResourceExists: false},
			wantDeployment:  trackedDeployment("ACTIVE", "DEPLOYING", "a"),
			wantLastHandled: "a",
			wantEvents:      []string{},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			appMock := &fake.MockApp{}
			appMock.Test(t)
			if tc.appGetErr {
				appMock.On("Get", guid).Return(fake.AppNil, fake.ErrNoResultReturned)
			} else {
				appMock.On("Get", guid).Return(observedCFApp("STARTED"), nil)
			}
			deployments := &fake.MockDeployment{}
			deployments.Test(t)
			if tc.deployments != nil {
				tc.deployments(deployments)
			}
			rec := &fakeRecorder{}
			writes := 0
			c := &external{
				kube: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(nil),
					MockStatusUpdate: func(context.Context, k8s.Object, ...k8s.SubResourceUpdateOption) error {
						writes++
						return tc.statusErr
					},
				},
				client:   &app.Client{AppClient: appMock, PushClient: newMockPush(), Deployments: deployments},
				recorder: rec,
			}

			obs, err := c.Observe(context.Background(), tc.mg)

			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("Observe() error = %v, want containing %q", err, tc.wantErr)
				}
			} else if err != nil {
				t.Fatalf("Observe() unexpected error: %v", err)
			} else if diff := cmp.Diff(tc.wantObs, obs); diff != "" {
				t.Errorf("Observe() obs: -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.wantDeployment, tc.mg.Status.AtProvider.Deployment); diff != "" {
				t.Errorf("status.atProvider.deployment: -want, +got:\n%s", diff)
			}
			if got := tc.mg.Status.AtProvider.LastHandledRestart; got != tc.wantLastHandled {
				t.Errorf("status.atProvider.lastHandledRestart = %q, want %q", got, tc.wantLastHandled)
			}
			if diff := cmp.Diff(tc.wantEvents, rec.kinds()); diff != "" {
				t.Errorf("events: -want, +got:\n%s", diff)
			}
			for _, s := range tc.wantMessage {
				if len(rec.events) == 0 || !strings.Contains(rec.events[0].Message, s) {
					t.Errorf("event message %v does not contain %q", rec.events, s)
				}
			}
			if writes != tc.wantWrites {
				t.Errorf("status writes = %d, want %d", writes, tc.wantWrites)
			}
			deployments.AssertExpectations(t)
		})
	}
}

// observedStatus is the status Observe leaves for observedCFApp: no spec drift.
func observedStatus(state string) modifier {
	return func(r *v1alpha1.App) {
		for _, m := range []modifier{
			withStatus(guid, state),
			withObservedName(name),
			withAppManifest("applications:\n- name: " + name),
			withObservedAnnotations(map[string]*string{
				"crossplane-kind": ptr.To("app.cloudfoundry.crossplane.io"),
				"crossplane-name": ptr.To("my-app"),
			}),
		} {
			m(r)
		}
	}
}

func TestUpdateRestart(t *testing.T) {
	stamped := map[string]*string{"crossplane-restart-sha256": ptr.To(stampA)}
	conflict := errors.New("conflict")
	// An earlier restart's deployment, still in status.
	previous := trackedDeployment("FINALIZED", "DEPLOYED", "z")
	previous.GUID = "0b7c4e2a-6f1d-4a8e-b3c5-9d2e7f1a4b6c"

	cases := map[string]struct {
		mg              *v1alpha1.App
		deployments     func(m *fake.MockDeployment)
		appUpdate       bool
		statusErr       error
		wantErr         string
		wantDeployment  *v1alpha1.AppDeploymentObservation
		wantLastHandled string
		wantEvents      []string
		wantWrites      int
	}{
		"StoppedAppSkips": {
			mg: restartApp(observedStatus("STOPPED"), withRestartAnnotation("a"), withLastHandledRestart("z"), withDeployment(trackedDeployment("FINALIZED", "DEPLOYED", "z"))),
			deployments: func(m *fake.MockDeployment) {
				m.On("List", mock.Anything).Return([]*cfresource.Deployment{}, nil)
			},
			wantDeployment:  trackedDeployment("FINALIZED", "DEPLOYED", "z"),
			wantLastHandled: "a",
			wantEvents:      []string{"Normal/RestartSkipped"},
			wantWrites:      1,
		},
		"StoppedAppSkipStatusWriteFails": {
			// Undone: the next reconcile skips again.
			mg: restartApp(observedStatus("STOPPED"), withRestartAnnotation("a"), withLastHandledRestart("z"), withDeployment(trackedDeployment("FINALIZED", "DEPLOYED", "z"))),
			deployments: func(m *fake.MockDeployment) {
				m.On("List", mock.Anything).Return([]*cfresource.Deployment{}, nil)
			},
			statusErr:       conflict,
			wantErr:         "conflict",
			wantDeployment:  trackedDeployment("FINALIZED", "DEPLOYED", "z"),
			wantLastHandled: "z",
			wantEvents:      []string{},
			wantWrites:      1,
		},
		"StartedAppStarts": {
			mg: restartApp(observedStatus("STARTED"), withRestartAnnotation("a")),
			deployments: func(m *fake.MockDeployment) {
				m.On("List", mock.Anything).Return([]*cfresource.Deployment{}, nil)
				m.On("Create", mock.MatchedBy(func(r *cfresource.DeploymentCreate) bool {
					return r.Strategy == "rolling" && r.Relationships.App.Data.GUID == guid &&
						r.Metadata != nil && r.Metadata.Annotations["crossplane-restart-sha256"] != nil &&
						*r.Metadata.Annotations["crossplane-restart-sha256"] == stampA
				})).Return(cfDeployment("ACTIVE", "DEPLOYING", stamped), nil).Once()
			},
			wantDeployment:  trackedDeployment("ACTIVE", "DEPLOYING", "a"),
			wantLastHandled: "a",
			wantEvents:      []string{"Normal/DeploymentStarted"},
			wantWrites:      1,
		},
		"CreateErrorKeepsPending": {
			mg: restartApp(observedStatus("STARTED"), withRestartAnnotation("a")),
			deployments: func(m *fake.MockDeployment) {
				m.On("List", mock.Anything).Return([]*cfresource.Deployment{}, nil)
				m.On("Create", mock.Anything).Return((*cfresource.Deployment)(nil), errBoom)
			},
			wantErr:    "boom",
			wantEvents: []string{},
		},
		"SpecChangesFirst": {
			mg: restartApp(observedStatus("STARTED"), specDrift, withRestartAnnotation("a")),
			deployments: func(m *fake.MockDeployment) {
				m.On("List", mock.Anything).Return([]*cfresource.Deployment{}, nil)
			},
			appUpdate:  true,
			wantEvents: []string{},
		},
		"AdoptStampedActive": {
			mg: restartApp(observedStatus("STARTED"), specDrift, withRestartAnnotation("a")),
			deployments: func(m *fake.MockDeployment) {
				m.On("List", mock.Anything).Return([]*cfresource.Deployment{cfDeployment("ACTIVE", "DEPLOYING", stamped)}, nil)
			},
			wantDeployment:  trackedDeployment("ACTIVE", "DEPLOYING", "a"),
			wantLastHandled: "a",
			wantEvents:      []string{},
			wantWrites:      1,
		},
		"AdoptStampedWhilePreviousTracked": {
			// Status save after Create lost while the previous restart is still tracked.
			mg: restartApp(observedStatus("STARTED"), withRestartAnnotation("a"), withLastHandledRestart("z"), withDeployment(previous)),
			deployments: func(m *fake.MockDeployment) {
				m.On("List", mock.Anything).Return([]*cfresource.Deployment{cfDeployment("ACTIVE", "DEPLOYING", stamped)}, nil)
			},
			wantDeployment:  trackedDeployment("ACTIVE", "DEPLOYING", "a"),
			wantLastHandled: "a",
			wantEvents:      []string{},
			wantWrites:      1,
		},
		"DeferToForeignActive": {
			mg: restartApp(observedStatus("STARTED"), specDrift, withRestartAnnotation("a")),
			deployments: func(m *fake.MockDeployment) {
				m.On("List", mock.Anything).Return([]*cfresource.Deployment{cfDeployment("ACTIVE", "DEPLOYING", nil)}, nil)
			},
			wantDeployment: trackedDeployment("ACTIVE", "DEPLOYING", ""),
			wantEvents:     []string{"Normal/RestartDeferred"},
			wantWrites:     1,
		},
		"DeferStatusWriteFails": {
			mg: restartApp(observedStatus("STARTED"), specDrift, withRestartAnnotation("a")),
			deployments: func(m *fake.MockDeployment) {
				m.On("List", mock.Anything).Return([]*cfresource.Deployment{cfDeployment("ACTIVE", "DEPLOYING", nil)}, nil)
			},
			statusErr:  conflict,
			wantErr:    "conflict",
			wantEvents: []string{},
			wantWrites: 1,
		},
		"AdoptStampedFinalized": {
			// Status save after Create lost, and the deployment has finished since.
			mg: restartApp(observedStatus("STARTED"), specDrift, withRestartAnnotation("a")),
			deployments: func(m *fake.MockDeployment) {
				m.On("List", mock.Anything).Return([]*cfresource.Deployment{cfDeployment("FINALIZED", "DEPLOYED", stamped)}, nil)
			},
			wantDeployment:  trackedDeployment("FINALIZED", "DEPLOYED", "a"),
			wantLastHandled: "a",
			wantEvents:      []string{"Normal/DeploymentSucceeded"},
			wantWrites:      1,
		},
		"AdoptStampedFinalizedStatusWriteFails": {
			mg: restartApp(observedStatus("STARTED"), specDrift, withRestartAnnotation("a")),
			deployments: func(m *fake.MockDeployment) {
				m.On("List", mock.Anything).Return([]*cfresource.Deployment{cfDeployment("FINALIZED", "DEPLOYED", stamped)}, nil)
			},
			statusErr:  conflict,
			wantErr:    "conflict",
			wantEvents: []string{},
			wantWrites: 1,
		},
		"TrackedStampRestartsAgain": {
			// A->B->A with B skipped on a stopped app: the newest deployment is
			// the tracked one stamped "a", already handled, so "a" restarts again.
			mg: restartApp(observedStatus("STARTED"), withRestartAnnotation("a"), withLastHandledRestart("b"), withDeployment(trackedDeployment("FINALIZED", "DEPLOYED", "a"))),
			deployments: func(m *fake.MockDeployment) {
				m.On("List", mock.Anything).Return([]*cfresource.Deployment{cfDeployment("FINALIZED", "DEPLOYED", stamped)}, nil)
				m.On("Create", mock.Anything).Return(cfDeployment("ACTIVE", "DEPLOYING", stamped), nil).Once()
			},
			wantDeployment:  trackedDeployment("ACTIVE", "DEPLOYING", "a"),
			wantLastHandled: "a",
			wantEvents:      []string{"Normal/DeploymentStarted"},
			wantWrites:      1,
		},
		"NewValueAfterFinal": {
			mg: restartApp(observedStatus("STARTED"), withRestartAnnotation("b"), withLastHandledRestart("a"), withDeployment(trackedDeployment("FINALIZED", "DEPLOYED", "a"))),
			deployments: func(m *fake.MockDeployment) {
				m.On("List", mock.Anything).Return([]*cfresource.Deployment{cfDeployment("FINALIZED", "DEPLOYED", stamped)}, nil)
				m.On("Create", mock.Anything).Return(cfDeployment("ACTIVE", "DEPLOYING", nil), nil).Once()
			},
			wantDeployment:  trackedDeployment("ACTIVE", "DEPLOYING", "b"),
			wantLastHandled: "b",
			wantEvents:      []string{"Normal/DeploymentStarted"},
			wantWrites:      1,
		},
		"ListError": {
			mg: restartApp(observedStatus("STARTED"), specDrift, withRestartAnnotation("a")),
			deployments: func(m *fake.MockDeployment) {
				m.On("List", mock.Anything).Return(([]*cfresource.Deployment)(nil), errBoom)
			},
			wantErr:    "boom",
			wantEvents: []string{},
		},
		"StatusWriteFails": {
			mg: restartApp(observedStatus("STARTED"), withRestartAnnotation("a")),
			deployments: func(m *fake.MockDeployment) {
				m.On("List", mock.Anything).Return([]*cfresource.Deployment{}, nil)
				m.On("Create", mock.Anything).Return(cfDeployment("ACTIVE", "DEPLOYING", stamped), nil).Once()
			},
			statusErr:       conflict,
			wantErr:         "conflict",
			wantDeployment:  trackedDeployment("ACTIVE", "DEPLOYING", "a"),
			wantLastHandled: "a",
			wantEvents:      []string{"Normal/DeploymentStarted"},
			wantWrites:      1,
		},
		"NoAnnotationUnchanged": {
			mg:         restartApp(observedStatus("STARTED"), specDrift),
			appUpdate:  true,
			wantEvents: []string{},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			appMock := &fake.MockApp{}
			appMock.Test(t)
			if tc.appUpdate {
				appMock.On("Update", guid).Return(observedCFApp("STARTED"), nil).Once()
			}
			deployments := &fake.MockDeployment{}
			deployments.Test(t)
			if tc.deployments != nil {
				tc.deployments(deployments)
			}
			writes := 0
			var written *v1alpha1.AppObservation
			rec := &fakeRecorder{}
			c := &external{
				kube: &test.MockClient{
					MockStatusUpdate: func(_ context.Context, obj k8s.Object, _ ...k8s.SubResourceUpdateOption) error {
						writes++
						written = obj.(*v1alpha1.App).Status.AtProvider.DeepCopy()
						return tc.statusErr
					},
				},
				client:   &app.Client{AppClient: appMock, PushClient: newMockPush(), Deployments: deployments},
				recorder: rec,
			}

			_, err := c.Update(context.Background(), tc.mg)

			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("Update() error = %v, want containing %q", err, tc.wantErr)
				}
			} else if err != nil {
				t.Fatalf("Update() unexpected error: %v", err)
			}
			if diff := cmp.Diff(tc.wantDeployment, tc.mg.Status.AtProvider.Deployment); diff != "" {
				t.Errorf("status.atProvider.deployment: -want, +got:\n%s", diff)
			}
			if got := tc.mg.Status.AtProvider.LastHandledRestart; got != tc.wantLastHandled {
				t.Errorf("status.atProvider.lastHandledRestart = %q, want %q", got, tc.wantLastHandled)
			}
			if diff := cmp.Diff(tc.wantEvents, rec.kinds()); diff != "" {
				t.Errorf("events: -want, +got:\n%s", diff)
			}
			if writes != tc.wantWrites {
				t.Errorf("status writes = %d, want %d", writes, tc.wantWrites)
			}
			// The immediate write must already carry the restart bookkeeping.
			if written != nil && tc.statusErr == nil {
				if diff := cmp.Diff(tc.wantDeployment, written.Deployment); diff != "" {
					t.Errorf("written status.atProvider.deployment: -want, +got:\n%s", diff)
				}
				if written.LastHandledRestart != tc.wantLastHandled {
					t.Errorf("written status.atProvider.lastHandledRestart = %q, want %q", written.LastHandledRestart, tc.wantLastHandled)
				}
			}
			appMock.AssertExpectations(t)
			deployments.AssertExpectations(t)
		})
	}
}
