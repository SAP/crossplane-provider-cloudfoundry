package test

import (
	"context"
	"errors"
	"fmt"
	"time"

	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients"
)

// ServiceCredentialBindingRotationCleanup contains operations scoped to the
// rotation E2E's owned binding, instance, and imported Kubernetes wrappers.
// External CF resources are only read for verification, never directly deleted.
type ServiceCredentialBindingRotationCleanup struct {
	CaptureBindingGUIDs func(context.Context) ([]string, error)
	CaptureInstanceGUID func(context.Context) (string, error)
	DeleteBinding       func(context.Context) error
	BindingExists       func(context.Context) (bool, error)
	BindingExistsInCF   func(context.Context, string) (bool, error)
	DeleteSecret        func(context.Context) error
	SecretExists        func(context.Context) (bool, error)
	DeleteInstance      func(context.Context) error
	InstanceExists      func(context.Context) (bool, error)
	DeleteImports       func(context.Context) error
	ImportsExist        func(context.Context) (bool, error)
	ExternalCheckError  func() error
}

// RunServiceCredentialBindingRotationCleanup detaches cleanup from assessment
// cancellation while preserving context values, and bounds all phases by one
// timeout. It waits for each owned managed resource to finalize before deleting
// its dependency. CF binding GUIDs are read-only verified; CF deletion remains
// the provider's responsibility. An unavailable CF read client is reported,
// but does not prevent safe deletion of the owned Kubernetes resources.
func RunServiceCredentialBindingRotationCleanup(
	parent context.Context,
	timeout, pollInterval time.Duration,
	ops ServiceCredentialBindingRotationCleanup,
) error {
	if timeout <= 0 {
		return errors.New("rotation cleanup timeout must be positive")
	}
	if pollInterval <= 0 {
		pollInterval = time.Second
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), timeout)
	defer cancel()

	state, errs := captureRotationCleanupState(ctx, ops)
	bindingGone, bindingErrs := deleteAndWaitForRotationBinding(ctx, pollInterval, ops)
	errs = append(errs, bindingErrs...)

	cfVerified, err := verifyRotationBindingGone(ctx, pollInterval, ops, state, bindingGone)
	appendCleanupError(&errs, "verify CF binding deletion", err)
	if bindingGone && cfVerified && ctx.Err() == nil {
		errs = append(errs, deleteRotationCleanupDependencies(ctx, pollInterval, ops, state.instanceGUID)...)
	}
	appendCleanupError(&errs, "cleanup deadline", ctx.Err())
	return errors.Join(errs...)
}

type rotationCleanupState struct {
	guids                    []string
	instanceGUID             string
	externalCheckUnavailable bool
}

func captureRotationCleanupState(ctx context.Context, ops ServiceCredentialBindingRotationCleanup) (rotationCleanupState, []error) {
	var state rotationCleanupState
	var errs []error
	if ops.CaptureBindingGUIDs == nil {
		appendCleanupError(&errs, "capture binding identities", errors.New("operation is not configured"))
	} else {
		var err error
		state.guids, err = ops.CaptureBindingGUIDs(ctx)
		appendCleanupError(&errs, "capture binding identities", err)
	}
	if ops.CaptureInstanceGUID == nil {
		appendCleanupError(&errs, "capture instance identity", errors.New("operation is not configured"))
	} else {
		var err error
		state.instanceGUID, err = ops.CaptureInstanceGUID(ctx)
		appendCleanupError(&errs, "capture instance identity", err)
	}
	if ops.ExternalCheckError != nil {
		err := ops.ExternalCheckError()
		appendCleanupError(&errs, "prepare CF verification", err)
		state.externalCheckUnavailable = err != nil
	}
	return state, errs
}

func deleteAndWaitForRotationBinding(ctx context.Context, interval time.Duration, ops ServiceCredentialBindingRotationCleanup) (bool, []error) {
	var errs []error
	appendCleanupError(&errs, "request binding deletion", callCleanup(ctx, ops.DeleteBinding))
	bindingGone, err := waitForCleanupAbsence(ctx, interval, ops.BindingExists)
	appendCleanupError(&errs, "wait for binding finalization", err)
	return bindingGone, errs
}

func verifyRotationBindingGone(ctx context.Context, interval time.Duration, ops ServiceCredentialBindingRotationCleanup, state rotationCleanupState, bindingGone bool) (bool, error) {
	if !bindingGone {
		return false, nil
	}
	switch {
	case state.externalCheckUnavailable:
		// The verification limitation is already recorded. The provider-managed
		// finalizer still permits safe cleanup of owned Kubernetes dependencies.
		return true, nil
	case ops.BindingExistsInCF == nil:
		if len(state.guids) == 0 {
			return true, nil
		}
		return false, errors.New("CF binding read operation is not configured")
	default:
		return verifyTrackedCFBindings(ctx, interval, ops.BindingExistsInCF, state.guids)
	}
}

func verifyTrackedCFBindings(ctx context.Context, interval time.Duration, exists func(context.Context, string) (bool, error), guids []string) (bool, error) {
	for _, guid := range uniqueNonEmpty(guids) {
		gone, err := waitForCleanupAbsence(ctx, interval, func(ctx context.Context) (bool, error) {
			return exists(ctx, guid)
		})
		if err != nil {
			return false, fmt.Errorf("binding identity %q: %w", guid, err)
		}
		if !gone {
			return false, nil
		}
	}
	return true, nil
}

func deleteRotationCleanupDependencies(ctx context.Context, interval time.Duration, ops ServiceCredentialBindingRotationCleanup, instanceGUID string) []error {
	var errs []error
	// Remove only the configured output Secret after its managed binding has
	// finalized. It is namespaced, unlike the cluster-scoped managed resources.
	appendCleanupError(&errs, "request connection Secret deletion", callCleanup(ctx, ops.DeleteSecret))
	_, err := waitForCleanupAbsence(ctx, interval, ops.SecretExists)
	appendCleanupError(&errs, "wait for connection Secret deletion", err)
	if ctx.Err() != nil {
		return errs
	}

	// Do not remove imported wrappers until the owned parent instance finalizes.
	appendCleanupError(&errs, "request instance deletion", callCleanup(ctx, ops.DeleteInstance))
	instanceGone, err := waitForCleanupAbsence(ctx, interval, ops.InstanceExists)
	if err != nil && instanceGUID != "" {
		err = fmt.Errorf("instance identity %q: %w", instanceGUID, err)
	}
	appendCleanupError(&errs, "wait for instance finalization", err)
	if !instanceGone || ctx.Err() != nil {
		return errs
	}
	appendCleanupError(&errs, "request imported-wrapper deletion", callCleanup(ctx, ops.DeleteImports))
	_, err = waitForCleanupAbsence(ctx, interval, ops.ImportsExist)
	appendCleanupError(&errs, "wait for imported-wrapper deletion", err)
	return errs
}

func appendCleanupError(errs *[]error, phase string, err error) {
	if err != nil {
		*errs = append(*errs, fmt.Errorf("%s: %w", phase, err))
	}
}

func callCleanup(ctx context.Context, action func(context.Context) error) error {
	if action == nil {
		return errors.New("operation is not configured")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return action(ctx)
}

func waitForCleanupAbsence(ctx context.Context, interval time.Duration, exists func(context.Context) (bool, error)) (bool, error) {
	if exists == nil {
		return false, errors.New("existence check is not configured")
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	absent := false
	err := wait.PollUntilContextCancel(ctx, interval, true, func(ctx context.Context) (bool, error) {
		present, err := exists(ctx)
		if err != nil {
			return false, err
		}
		absent = !present
		return absent, nil
	})
	return absent, err
}

// ValidateRetiredKeyDeletionObservation classifies one status-then-CF cleanup
// sample. A status record disappearing while CF still returns the binding is a
// tracking-loss error, not a transient wait or successful collection.
func ValidateRetiredKeyDeletionObservation(guid string, cfPresent bool, retired []*v1alpha1.SCBResource) (bool, error) {
	if guid == "" {
		return false, errors.New("retired binding identity is empty")
	}
	tracked := false
	for _, key := range retired {
		if key != nil && key.GUID == guid {
			tracked = true
			break
		}
	}
	if !tracked && cfPresent {
		return false, errors.New("retired binding is still present in Cloud Foundry but missing from status")
	}
	return !cfPresent && !tracked, nil
}

// RotationRetiredKeyCollected requires both Cloud Foundry absence and status GC.
func RotationRetiredKeyCollected(guid string, cfPresent bool, retired []*v1alpha1.SCBResource) bool {
	collected, err := ValidateRetiredKeyDeletionObservation(guid, cfPresent, retired)
	return err == nil && collected
}

func uniqueNonEmpty(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	unique := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	return unique
}

// ServiceCredentialBindingExistsInCF confirms a tracked external key is still
// present. Both CF not-found error classes mean the owned key is gone.
func ServiceCredentialBindingExistsInCF(ctx context.Context, getter interface {
	Get(context.Context, string) (*cfresource.ServiceCredentialBinding, error)
}, guid string) (bool, error) {
	binding, err := getter.Get(ctx, guid)
	if clients.ErrorIsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return true, err
	}
	if binding == nil {
		return true, errors.New("cloud foundry returned no binding without a not-found response")
	}
	return true, nil
}

// IgnoreRotationCleanupNotFound normalizes Kubernetes NotFound responses from
// deletes; existence is still polled so an already-absent resource is confirmed.
func IgnoreRotationCleanupNotFound(err error) error {
	if kubeerrors.IsNotFound(err) {
		return nil
	}
	return err
}
