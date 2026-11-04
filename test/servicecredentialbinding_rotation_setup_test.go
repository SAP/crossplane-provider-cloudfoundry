package test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
)

func TestRotationSetupOrdersPhasesAndPollsPending(t *testing.T) {
	var calls []string
	instanceProbeCount, bindingProbeCount := 0, 0
	ops := setupTestOperations()
	ops.PhaseTimeout = time.Second
	ops.PollInterval = time.Millisecond
	ops.ApplyImports = func(context.Context) error { calls = append(calls, "apply-imports"); return nil }
	ops.ImportsReady = func(context.Context) (bool, error) { calls = append(calls, "probe-imports"); return true, nil }
	ops.ApplyInstance = func(context.Context) error {
		calls = append(calls, "apply-instance")
		if !containsString(calls, "probe-imports") {
			t.Fatal("instance applied before imports were probed")
		}
		return nil
	}
	ops.InstanceReady = func(context.Context) (bool, error) {
		calls = append(calls, "probe-instance")
		instanceProbeCount++
		return instanceProbeCount > 1, nil
	}
	ops.ApplyBinding = func(context.Context) error {
		calls = append(calls, "apply-binding")
		if instanceProbeCount <= 1 {
			t.Fatal("binding applied before service instance became ready")
		}
		return nil
	}
	ops.BindingReady = func(context.Context) (bool, error) {
		calls = append(calls, "probe-binding")
		bindingProbeCount++
		return bindingProbeCount > 1, nil
	}
	if err := RunServiceCredentialBindingRotationSetup(context.Background(), ops); err != nil {
		t.Fatalf("RunServiceCredentialBindingRotationSetup() error = %v", err)
	}
	wantOrder := []string{"apply-imports", "probe-imports", "apply-instance", "probe-instance", "apply-binding", "probe-binding"}
	last := -1
	for _, expected := range wantOrder {
		found := -1
		for i := last + 1; i < len(calls); i++ {
			if calls[i] == expected {
				found = i
				break
			}
		}
		if found < 0 {
			t.Fatalf("phase order = %v, missing %q after index %d", calls, expected, last)
		}
		last = found
	}
}

func TestRotationSetupStopsAtPhaseFailures(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*RotationSetupOperations)
		phase     string
		kind      RotationSetupFailureKind
	}{
		{"import apply", func(o *RotationSetupOperations) {
			o.ApplyImports = func(context.Context) error {
				return apierrors.NewForbidden(schema.GroupResource{}, "x", errors.New("sensitive-sentinel"))
			}
		}, "imports", RotationSetupForbidden},
		{"instance apply", func(o *RotationSetupOperations) {
			o.ImportsReady = func(context.Context) (bool, error) { return true, nil }
			o.ApplyInstance = func(context.Context) error { return errors.New("sensitive-sentinel") }
		}, "service-instance", RotationSetupApplyFailed},
		{"instance probe", func(o *RotationSetupOperations) {
			o.ImportsReady = func(context.Context) (bool, error) { return true, nil }
			o.InstanceReady = func(context.Context) (bool, error) {
				return false, &RotationSetupError{Kind: RotationSetupProvisioningFailed}
			}
		}, "service-instance", RotationSetupProvisioningFailed},
		{"binding apply", func(o *RotationSetupOperations) {
			o.ImportsReady = func(context.Context) (bool, error) { return true, nil }
			o.InstanceReady = func(context.Context) (bool, error) { return true, nil }
			o.ApplyBinding = func(context.Context) error { return errors.New("sensitive-sentinel") }
		}, "binding", RotationSetupApplyFailed},
		{"binding paused", func(o *RotationSetupOperations) {
			o.ImportsReady = func(context.Context) (bool, error) { return true, nil }
			o.InstanceReady = func(context.Context) (bool, error) { return true, nil }
			o.BindingReady = func(context.Context) (bool, error) { return false, &RotationSetupError{Kind: RotationSetupPaused} }
		}, "binding", RotationSetupPaused},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops := setupTestOperations()
			tt.configure(&ops)
			ops.PhaseTimeout = 15 * time.Millisecond
			ops.PollInterval = time.Millisecond
			bindingApplyCalls := 0
			if tt.phase != "binding" {
				original := ops.ApplyBinding
				ops.ApplyBinding = func(ctx context.Context) error { bindingApplyCalls++; return original(ctx) }
			}
			err := RunServiceCredentialBindingRotationSetup(context.Background(), ops)
			var setupErr *RotationSetupError
			if !errors.As(err, &setupErr) || setupErr.Phase != tt.phase || setupErr.Kind != tt.kind {
				t.Fatalf("error = %#v, want phase %q kind %q", err, tt.phase, tt.kind)
			}
			if bindingApplyCalls != 0 {
				t.Fatalf("binding applied after earlier failure: %d", bindingApplyCalls)
			}
			if strings.Contains(err.Error(), "sensitive-sentinel") {
				t.Fatalf("credential-like error leaked: %v", err)
			}
		})
	}
}

func TestRotationSetupPhaseTimeoutDoesNotExtendAssessmentDeadline(t *testing.T) {
	ops := setupTestOperations()
	ops.PhaseTimeout = time.Second
	ops.PollInterval = time.Millisecond
	ops.ImportsReady = func(context.Context) (bool, error) { return false, nil }
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Millisecond)
	defer cancel()
	started := time.Now()
	err := RunServiceCredentialBindingRotationSetup(ctx, ops)
	var setupErr *RotationSetupError
	if !errors.As(err, &setupErr) || setupErr.Kind != RotationSetupTimedOut {
		t.Fatalf("error = %v, want bounded timeout", err)
	}
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		t.Fatalf("setup exceeded assessment deadline by too much: %s", elapsed)
	}
}

func TestRotationSetupTransientProbeErrorsRecoverAndClassifyPermanent(t *testing.T) {
	t.Run("transient not found recovers", func(t *testing.T) {
		ops := setupTestOperations()
		ops.PhaseTimeout, ops.PollInterval = time.Second, time.Millisecond
		attempts := 0
		ops.ImportsReady = func(context.Context) (bool, error) {
			attempts++
			if attempts == 1 {
				return false, apierrors.NewNotFound(schema.GroupResource{}, "rotation-org")
			}
			return true, nil
		}
		if err := RunServiceCredentialBindingRotationSetup(context.Background(), ops); err != nil {
			t.Fatalf("RunServiceCredentialBindingRotationSetup() error = %v", err)
		}
		if attempts < 2 {
			t.Fatalf("imports probe attempts = %d, want retry", attempts)
		}
	})

	t.Run("permanent forbidden probe stops", func(t *testing.T) {
		ops := setupTestOperations()
		ops.PhaseTimeout, ops.PollInterval = time.Second, time.Millisecond
		ops.ImportsReady = func(context.Context) (bool, error) {
			return false, apierrors.NewForbidden(schema.GroupResource{}, "rotation-org", errors.New("secret sentinel"))
		}
		err := RunServiceCredentialBindingRotationSetup(context.Background(), ops)
		var setupErr *RotationSetupError
		if !errors.As(err, &setupErr) || setupErr.Kind != RotationSetupForbidden {
			t.Fatalf("error = %v, want forbidden setup error", err)
		}
		if strings.Contains(err.Error(), "sentinel") {
			t.Fatalf("sensitive probe error leaked: %v", err)
		}
	})
}

func TestRotationSetupCancellationConfigurationAndDiagnosticBounds(t *testing.T) {
	t.Run("already canceled makes no calls", func(t *testing.T) {
		ops := setupTestOperations()
		ops.PhaseTimeout = time.Second
		calls := 0
		ops.ApplyImports = func(context.Context) error { calls++; return nil }
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := RunServiceCredentialBindingRotationSetup(ctx, ops)
		if err == nil || calls != 0 {
			t.Fatalf("error=%v calls=%d, want canceled without callbacks", err, calls)
		}
	})

	t.Run("cancellation during a phase prevents downstream apply", func(t *testing.T) {
		ops := setupTestOperations()
		ops.PhaseTimeout = time.Second
		ctx, cancel := context.WithCancel(context.Background())
		instanceApplied := false
		ops.ApplyImports = func(context.Context) error { cancel(); return nil }
		ops.ApplyInstance = func(context.Context) error { instanceApplied = true; return nil }
		err := RunServiceCredentialBindingRotationSetup(ctx, ops)
		if err == nil || instanceApplied {
			t.Fatalf("error=%v instanceApplied=%t, want stopped cancellation", err, instanceApplied)
		}
	})

	t.Run("missing callback fails safely", func(t *testing.T) {
		ops := setupTestOperations()
		ops.ApplyBinding = nil
		err := RunServiceCredentialBindingRotationSetup(context.Background(), ops)
		var setupErr *RotationSetupError
		if !errors.As(err, &setupErr) || setupErr.Kind != RotationSetupInvalidConfiguration {
			t.Fatalf("error = %v, want invalid configuration", err)
		}
	})

	t.Run("diagnostic is bounded and cannot mask phase error", func(t *testing.T) {
		ops := setupTestOperations()
		ops.ApplyImports = func(context.Context) error { return errors.New("sensitive sentinel") }
		called := false
		ops.OnFailureRead = func(ctx context.Context, _ string) {
			called = true
			deadline, ok := ctx.Deadline()
			if !ok || time.Until(deadline) > 5*time.Second {
				t.Error("diagnostic context is not bounded")
			}
		}
		err := RunServiceCredentialBindingRotationSetup(context.Background(), ops)
		var setupErr *RotationSetupError
		if !called || !errors.As(err, &setupErr) || setupErr.Kind != RotationSetupApplyFailed || strings.Contains(err.Error(), "sensitive") {
			t.Fatalf("diagnostics=%t error=%v", called, err)
		}
	})
}

func TestRotationSetupTypedReadiness(t *testing.T) {
	if ready, err := RotationOrganizationSpaceReady(nil, nil); ready || err != nil {
		t.Fatalf("nil imports readiness = (%t, %v), want pending", ready, err)
	}
	readyConditions := []xpv2.Condition{
		{Type: xpv2.TypeReady, Status: corev1.ConditionTrue},
		{Type: xpv2.TypeSynced, Status: corev1.ConditionTrue},
	}
	id := "instance-guid"
	instance := &v1alpha1.ServiceInstance{}
	instance.Status.AtProvider.ID = &id
	instance.Status.AtProvider.LastOperation = v1alpha1.LastOperation{Type: v1alpha1.LastOperationCreate, State: v1alpha1.LastOperationSucceeded}
	instance.Status.SetConditions(readyConditions...)
	if ready, err := RotationServiceInstanceReady(instance); err != nil || !ready {
		t.Fatalf("successful instance readiness = (%t, %v)", ready, err)
	}

	orgID := "org-guid"
	org := &v1alpha1.Organization{}
	org.Status.AtProvider.ID = &orgID
	space := &v1alpha1.Space{}
	space.Status.AtProvider.ID = "space-guid"
	org.Status.SetConditions(readyConditions...)
	space.Status.SetConditions(readyConditions...)
	if ready, err := RotationOrganizationSpaceReady(org, space); err != nil || !ready {
		t.Fatalf("successful import readiness = (%t, %v)", ready, err)
	}
	space.Status.AtProvider.ID = ""
	if ready, err := RotationOrganizationSpaceReady(org, space); err != nil || ready {
		t.Fatalf("missing space ID readiness = (%t, %v), want pending", ready, err)
	}

	instance.Status.AtProvider.ID = nil
	if ready, err := RotationServiceInstanceReady(instance); err != nil || ready {
		t.Fatalf("nil instance ID readiness = (%t, %v), want pending", ready, err)
	}
	instance.Status.AtProvider.ID = &id
	instance.Status.AtProvider.State = v1alpha1.LastOperationFailed
	if _, err := RotationServiceInstanceReady(instance); !hasSetupKind(err, RotationSetupProvisioningFailed) {
		t.Fatalf("failed operation error = %v, want provisioning failure", err)
	}

	binding := &v1alpha1.ServiceCredentialBinding{}
	binding.Status.AtProvider.GUID = "binding-guid"
	binding.Status.AtProvider.LastOperation = &v1alpha1.LastOperation{Type: v1alpha1.LastOperationCreate, State: v1alpha1.LastOperationSucceeded}
	binding.Status.SetConditions(readyConditions...)
	if ready, err := RotationServiceCredentialBindingReady(binding); err != nil || !ready {
		t.Fatalf("successful binding readiness = (%t, %v)", ready, err)
	}
	binding.Annotations = map[string]string{meta.AnnotationKeyReconciliationPaused: "true"}
	if _, err := RotationServiceCredentialBindingReady(binding); !hasSetupKind(err, RotationSetupPaused) {
		t.Fatalf("paused binding error = %v, want paused", err)
	}
	binding.Annotations = map[string]string{"servicecredentialbinding.cloudfoundry.crossplane.io/max-retry-exceeded": "now"}
	if _, err := RotationServiceCredentialBindingReady(binding); !hasSetupKind(err, RotationSetupCreateExhausted) {
		t.Fatalf("exhausted binding error = %v, want create exhausted", err)
	}
	binding.Annotations = nil
	binding.Status.SetConditions(
		xpv2.Condition{Type: xpv2.TypeReady, Status: corev1.ConditionFalse},
		xpv2.Condition{Type: xpv2.TypeSynced, Status: corev1.ConditionFalse, Reason: xpv2.ReasonReconcileError, Message: "forbidden: sensitive-sentinel"},
	)
	if _, err := RotationServiceCredentialBindingReady(binding); !hasSetupKind(err, RotationSetupForbidden) || strings.Contains(err.Error(), "sentinel") {
		t.Fatalf("permanent reconcile error = %v, want sanitized forbidden failure", err)
	}
}

func setupTestOperations() RotationSetupOperations {
	return RotationSetupOperations{
		ApplyImports:  func(context.Context) error { return nil },
		ImportsReady:  func(context.Context) (bool, error) { return true, nil },
		ApplyInstance: func(context.Context) error { return nil },
		InstanceReady: func(context.Context) (bool, error) { return true, nil },
		ApplyBinding:  func(context.Context) error { return nil },
		BindingReady:  func(context.Context) (bool, error) { return true, nil },
		PhaseTimeout:  time.Second,
		PollInterval:  time.Millisecond,
	}
}

func hasSetupKind(err error, kind RotationSetupFailureKind) bool {
	var setupErr *RotationSetupError
	return errors.As(err, &setupErr) && setupErr.Kind == kind
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
