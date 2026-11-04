package test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
)

// RotationFixtureIdentityChecks records only safe identity-match booleans. It
// intentionally does not compare Go types: fixture decoding may use Unstructured
// while the typed resource client reads into a scheme-allocated API object.
type RotationFixtureIdentityChecks struct {
	ManifestGVKMatches bool
	FetchedGVKMatches  bool
	NameMatches        bool
	NamespaceMatches   bool
}

func (c RotationFixtureIdentityChecks) Matches() bool {
	return c.ManifestGVKMatches && c.FetchedGVKMatches && c.NameMatches && c.NamespaceMatches
}

// CheckRotationFixtureIdentity validates identity without requiring the typed
// Kubernetes GET result to carry TypeMeta. An empty fetched GVK is acceptable;
// the caller constructs that object from the expected GVK using its API scheme.
func CheckRotationFixtureIdentity(expected schema.GroupVersionKind, desired, current runtime.Object) RotationFixtureIdentityChecks {
	checks := RotationFixtureIdentityChecks{}
	if desired == nil || current == nil {
		return checks
	}
	checks.ManifestGVKMatches = desired.GetObjectKind().GroupVersionKind() == expected
	currentGVK := current.GetObjectKind().GroupVersionKind()
	checks.FetchedGVKMatches = currentGVK.Empty() || currentGVK == expected
	desiredMeta, desiredOK := desired.(metav1.Object)
	currentMeta, currentOK := current.(metav1.Object)
	if desiredOK && currentOK {
		checks.NameMatches = desiredMeta.GetName() == currentMeta.GetName()
		checks.NamespaceMatches = desiredMeta.GetNamespace() == currentMeta.GetNamespace()
	}
	return checks
}

// RotationSetupFailureKind is a credential-safe setup failure category.
type RotationSetupFailureKind string

const (
	rotationSetupResourcePhase = "resource"

	RotationSetupApplyFailed             RotationSetupFailureKind = "apply-failed"
	RotationSetupApplyConflict           RotationSetupFailureKind = "apply-conflict"
	RotationSetupApplyNotFound           RotationSetupFailureKind = "apply-not-found"
	RotationSetupApplyTimedOut           RotationSetupFailureKind = "apply-timeout"
	RotationSetupApplyRateLimited        RotationSetupFailureKind = "apply-rate-limited"
	RotationSetupApplyUnavailable        RotationSetupFailureKind = "apply-unavailable"
	RotationSetupApplyAPIError           RotationSetupFailureKind = "apply-api-error"
	RotationSetupFixtureDecodeFailed     RotationSetupFailureKind = "fixture-decode-failed"
	RotationSetupFixtureKindMissing      RotationSetupFailureKind = "fixture-kind-missing"
	RotationSetupFixtureConversionFailed RotationSetupFailureKind = "fixture-conversion-failed"
	RotationSetupFixtureIdentityMismatch RotationSetupFailureKind = "fixture-identity-mismatch"
	RotationSetupFixtureSpecMismatch     RotationSetupFailureKind = "fixture-spec-mismatch"
	RotationSetupForbidden               RotationSetupFailureKind = "forbidden"
	RotationSetupInvalid                 RotationSetupFailureKind = "invalid"
	RotationSetupUnauthorized            RotationSetupFailureKind = "unauthorized"
	RotationSetupProvisioningFailed      RotationSetupFailureKind = "provisioning-failed"
	RotationSetupPaused                  RotationSetupFailureKind = "paused"
	RotationSetupCreateExhausted         RotationSetupFailureKind = "create-exhausted"
	RotationSetupReadFailed              RotationSetupFailureKind = "read-failed"
	RotationSetupTimedOut                RotationSetupFailureKind = "timeout"
	RotationSetupInvalidConfiguration    RotationSetupFailureKind = "invalid-configuration"
)

// RotationSetupError deliberately omits the underlying API error text, which may
// contain broker parameters or other sensitive data.
type RotationSetupError struct {
	Phase          string
	Kind           RotationSetupFailureKind
	ResourceKind   string
	ResourceName   string
	APICode        int32
	IdentityChecks *RotationFixtureIdentityChecks
}

func (e *RotationSetupError) Error() string {
	message := fmt.Sprintf("service-key rotation setup phase %q failed (%s)", e.Phase, e.Kind)
	if e.ResourceKind != "" {
		message += fmt.Sprintf(" resource=%s", e.ResourceKind)
		if e.ResourceName != "" {
			message += "/" + e.ResourceName
		}
	}
	if e.APICode != 0 {
		message += fmt.Sprintf(" api-code=%d", e.APICode)
	}
	if e.IdentityChecks != nil {
		checks := e.IdentityChecks
		message += fmt.Sprintf(" identity-checks={manifest-gvk:%t,fetched-gvk:%t,name:%t,namespace:%t}", checks.ManifestGVKMatches, checks.FetchedGVKMatches, checks.NameMatches, checks.NamespaceMatches)
	}
	return message
}

// RotationSetupApplyFailure converts an arbitrary apply error into a safe,
// allowlisted category. It deliberately discards the error text and status body.
func RotationSetupApplyFailure(err error, resourceKind, resourceName string) *RotationSetupError {
	var setupErr *RotationSetupError
	if errors.As(err, &setupErr) {
		failure := *setupErr
		if failure.ResourceKind == "" {
			failure.ResourceKind = resourceKind
		}
		if failure.ResourceName == "" {
			failure.ResourceName = resourceName
		}
		return &failure
	}
	failure := &RotationSetupError{
		Kind:         RotationSetupApplyFailed,
		ResourceKind: resourceKind,
		ResourceName: resourceName,
	}
	var apiStatus apierrors.APIStatus
	if errors.As(err, &apiStatus) {
		failure.APICode = apiStatus.Status().Code
	}
	failure.Kind = rotationSetupApplyFailureKind(err, failure.APICode)
	return failure
}

func rotationSetupApplyFailureKind(err error, apiCode int32) RotationSetupFailureKind {
	switch {
	case apierrors.IsForbidden(err):
		return RotationSetupForbidden
	case apierrors.IsUnauthorized(err):
		return RotationSetupUnauthorized
	case apierrors.IsInvalid(err):
		return RotationSetupInvalid
	case apierrors.IsConflict(err):
		return RotationSetupApplyConflict
	case apierrors.IsNotFound(err):
		return RotationSetupApplyNotFound
	case apierrors.IsTimeout(err), apierrors.IsServerTimeout(err):
		return RotationSetupApplyTimedOut
	case apierrors.IsTooManyRequests(err):
		return RotationSetupApplyRateLimited
	case apierrors.IsServiceUnavailable(err), apierrors.IsInternalError(err):
		return RotationSetupApplyUnavailable
	case apiCode != 0:
		return RotationSetupApplyAPIError
	default:
		return RotationSetupApplyFailed
	}
}

// RotationSetupOperations describes the dependency order for one assessment.
// Each readiness probe returns false while the resource is legitimately pending.
type RotationSetupOperations struct {
	ApplyImports  func(context.Context) error
	ImportsReady  func(context.Context) (bool, error)
	ApplyInstance func(context.Context) error
	InstanceReady func(context.Context) (bool, error)
	ApplyBinding  func(context.Context) error
	BindingReady  func(context.Context) (bool, error)
	PhaseTimeout  time.Duration
	PollInterval  time.Duration
	OnProgress    func(phase, state string, elapsed time.Duration)
	OnFailureRead func(context.Context, string)
}

// RunServiceCredentialBindingRotationSetup applies and waits for each owned
// dependency in order. Phase caps never extend the caller's assessment deadline.
func RunServiceCredentialBindingRotationSetup(ctx context.Context, ops RotationSetupOperations) error {
	if err := validateRotationSetupOperations(ops); err != nil {
		return err
	}
	if ops.PollInterval <= 0 {
		ops.PollInterval = time.Second
	}
	phases := []rotationSetupPhase{
		{name: "imports", apply: ops.ApplyImports, ready: ops.ImportsReady},
		{name: "service-instance", apply: ops.ApplyInstance, ready: ops.InstanceReady},
		{name: "binding", apply: ops.ApplyBinding, ready: ops.BindingReady},
	}
	for _, phase := range phases {
		if err := ctx.Err(); err != nil {
			return rotationSetupFailure(phase.name, RotationSetupTimedOut, ops.OnFailureRead, ctx)
		}
		started := time.Now()
		progressRotationSetup(ops, phase.name, "applying", started)
		if err := phase.apply(ctx); err != nil {
			if ctx.Err() != nil {
				return rotationSetupFailure(phase.name, RotationSetupTimedOut, ops.OnFailureRead, ctx)
			}
			failure := rotationSetupErrorFor(phase.name, err, false)
			rotationSetupDiagnostics(ctx, phase.name, ops.OnFailureRead)
			return failure
		}
		progressRotationSetup(ops, phase.name, "pending", started)
		phaseCtx, cancel := context.WithTimeout(ctx, ops.PhaseTimeout)
		err := waitForRotationSetupPhase(phaseCtx, phase, ops)
		cancel()
		if err != nil {
			rotationSetupDiagnostics(ctx, phase.name, ops.OnFailureRead)
			return err
		}
		progressRotationSetup(ops, phase.name, "ready", started)
	}
	return nil
}

type rotationSetupPhase struct {
	name  string
	apply func(context.Context) error
	ready func(context.Context) (bool, error)
}

func validateRotationSetupOperations(ops RotationSetupOperations) error {
	if ops.PhaseTimeout <= 0 {
		return &RotationSetupError{Phase: "configuration", Kind: RotationSetupInvalidConfiguration}
	}
	callbacks := []struct {
		name    string
		missing bool
	}{
		{"apply-imports", ops.ApplyImports == nil}, {"probe-imports", ops.ImportsReady == nil},
		{"apply-instance", ops.ApplyInstance == nil}, {"probe-instance", ops.InstanceReady == nil},
		{"apply-binding", ops.ApplyBinding == nil}, {"probe-binding", ops.BindingReady == nil},
	}
	for _, callback := range callbacks {
		if callback.missing {
			return &RotationSetupError{Phase: callback.name, Kind: RotationSetupInvalidConfiguration}
		}
	}
	return nil
}

func waitForRotationSetupPhase(ctx context.Context, phase rotationSetupPhase, ops RotationSetupOperations) error {
	for {
		if ctx.Err() != nil {
			return &RotationSetupError{Phase: phase.name, Kind: RotationSetupTimedOut}
		}
		ready, err := rotationSetupProbe(ctx, phase)
		if err != nil {
			return err
		}
		if ready {
			return nil
		}
		if err := waitRotationSetupInterval(ctx, ops.PollInterval); err != nil {
			return &RotationSetupError{Phase: phase.name, Kind: RotationSetupTimedOut}
		}
	}
}

func rotationSetupProbe(ctx context.Context, phase rotationSetupPhase) (bool, error) {
	ready, err := phase.ready(ctx)
	if err == nil {
		return ready, nil
	}
	if ctx.Err() != nil {
		return false, &RotationSetupError{Phase: phase.name, Kind: RotationSetupTimedOut}
	}
	kind, pending := classifyRotationSetupProbeError(err)
	if pending {
		return false, nil
	}
	return false, &RotationSetupError{Phase: phase.name, Kind: kind}
}

func waitRotationSetupInterval(ctx context.Context, interval time.Duration) error {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func classifyRotationSetupProbeError(err error) (RotationSetupFailureKind, bool) {
	var setupErr *RotationSetupError
	if errors.As(err, &setupErr) {
		return setupErr.Kind, false
	}
	switch {
	case apierrors.IsNotFound(err), apierrors.IsTimeout(err), apierrors.IsServerTimeout(err), apierrors.IsTooManyRequests(err), apierrors.IsServiceUnavailable(err), apierrors.IsInternalError(err):
		return "", true
	case apierrors.IsForbidden(err):
		return RotationSetupForbidden, false
	case apierrors.IsUnauthorized(err):
		return RotationSetupUnauthorized, false
	case apierrors.IsInvalid(err):
		return RotationSetupInvalid, false
	default:
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return "", true
		}
		return RotationSetupReadFailed, false
	}
}

func rotationSetupErrorFor(phase string, err error, probe bool) *RotationSetupError {
	var setupErr *RotationSetupError
	if errors.As(err, &setupErr) {
		return &RotationSetupError{
			Phase:          phase,
			Kind:           setupErr.Kind,
			ResourceKind:   setupErr.ResourceKind,
			ResourceName:   setupErr.ResourceName,
			APICode:        setupErr.APICode,
			IdentityChecks: setupErr.IdentityChecks,
		}
	}
	if !probe {
		failure := RotationSetupApplyFailure(err, "", "")
		failure.Phase = phase
		return failure
	}
	kind, _ := classifyRotationSetupProbeError(err)
	return &RotationSetupError{Phase: phase, Kind: kind}
}

func rotationSetupFailure(phase string, kind RotationSetupFailureKind, diagnose func(context.Context, string), ctx context.Context) *RotationSetupError {
	failure := &RotationSetupError{Phase: phase, Kind: kind}
	rotationSetupDiagnostics(ctx, phase, diagnose)
	return failure
}

func rotationSetupDiagnostics(ctx context.Context, phase string, diagnose func(context.Context, string)) {
	if diagnose == nil || ctx.Err() != nil {
		return
	}
	diagnosticCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	diagnose(diagnosticCtx, phase)
}

// RotationOrganizationSpaceReady requires both imported wrappers to resolve to
// existing CF identities and be successfully observed, without mutating either.
func RotationOrganizationSpaceReady(org *v1alpha1.Organization, space *v1alpha1.Space) (bool, error) {
	if org == nil || space == nil {
		return false, nil
	}
	orgReady, err := rotationManagedReady(org, org.Status.AtProvider.ID != nil && *org.Status.AtProvider.ID != "", false, nil)
	if err != nil || !orgReady {
		return orgReady, err
	}
	return rotationManagedReady(space, space.Status.AtProvider.ID != "", false, nil)
}

// RotationServiceInstanceReady requires the CF instance identity, successful
// create operation, and ready/synced conditions.
func RotationServiceInstanceReady(instance *v1alpha1.ServiceInstance) (bool, error) {
	if instance == nil {
		return false, nil
	}
	return rotationManagedReady(instance, instance.Status.AtProvider.ID != nil && *instance.Status.AtProvider.ID != "", true, &instance.Status.AtProvider.LastOperation)
}

// RotationServiceCredentialBindingReady requires a created CF binding. Secret
// publication is deliberately checked separately by the existing read-back helper.
func RotationServiceCredentialBindingReady(binding *v1alpha1.ServiceCredentialBinding) (bool, error) {
	if binding == nil {
		return false, nil
	}
	return rotationManagedReady(binding, binding.Status.AtProvider.GUID != "", true, binding.Status.AtProvider.LastOperation)
}

func rotationManagedReady(managed resource.Managed, hasIdentity, requireCreate bool, operation *v1alpha1.LastOperation) (bool, error) {
	if kind := rotationManagedFailure(managed, operation); kind != "" {
		return false, &RotationSetupError{Phase: rotationSetupResourcePhase, Kind: kind}
	}
	synced := managed.GetCondition(xpv2.TypeSynced)
	ready := managed.GetCondition(xpv2.TypeReady)
	if !hasIdentity || ready.Status != corev1.ConditionTrue || synced.Status != corev1.ConditionTrue {
		return false, nil
	}
	if requireCreate && (operation == nil || operation.Type != v1alpha1.LastOperationCreate || operation.State != v1alpha1.LastOperationSucceeded) {
		return false, nil
	}
	return true, nil
}

func rotationManagedFailure(managed resource.Managed, operation *v1alpha1.LastOperation) RotationSetupFailureKind {
	annotations := managed.GetAnnotations()
	if annotations[meta.AnnotationKeyReconciliationPaused] == "true" {
		return RotationSetupPaused
	}
	if _, exhausted := annotations["servicecredentialbinding.cloudfoundry.crossplane.io/max-retry-exceeded"]; exhausted {
		return RotationSetupCreateExhausted
	}
	if operation != nil && operation.State == v1alpha1.LastOperationFailed {
		return RotationSetupProvisioningFailed
	}
	synced := managed.GetCondition(xpv2.TypeSynced)
	if synced.Status == corev1.ConditionFalse && synced.Reason == xpv2.ReasonReconcilePaused {
		return RotationSetupPaused
	}
	if synced.Status == corev1.ConditionFalse && synced.Reason == xpv2.ReasonReconcileError {
		return rotationSetupConditionFailureKind(synced.Message)
	}
	return ""
}

func rotationSetupConditionFailureKind(message string) RotationSetupFailureKind {
	message = strings.ToLower(message)
	switch {
	case strings.Contains(message, "forbidden"):
		return RotationSetupForbidden
	case strings.Contains(message, "unauthorized"):
		return RotationSetupUnauthorized
	case strings.Contains(message, "invalid"):
		return RotationSetupInvalid
	default:
		return ""
	}
}

func progressRotationSetup(ops RotationSetupOperations, phase, state string, started time.Time) {
	if ops.OnProgress != nil {
		ops.OnProgress(phase, state, time.Since(started))
	}
}
