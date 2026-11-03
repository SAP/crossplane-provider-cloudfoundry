package test

import (
	"context"
	"errors"
	"fmt"
	"time"

	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"

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

	var errs []error
	addErr := func(phase string, err error) {
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", phase, err))
		}
	}

	var guids []string
	if ops.CaptureBindingGUIDs == nil {
		addErr("capture binding identities", errors.New("operation is not configured"))
	} else {
		var err error
		guids, err = ops.CaptureBindingGUIDs(ctx)
		addErr("capture binding identities", err)
	}
	externalCheckUnavailable := false
	instanceGUID := ""
	if ops.CaptureInstanceGUID == nil {
		addErr("capture instance identity", errors.New("operation is not configured"))
	} else {
		var err error
		instanceGUID, err = ops.CaptureInstanceGUID(ctx)
		addErr("capture instance identity", err)
	}
	if ops.ExternalCheckError != nil {
		externalErr := ops.ExternalCheckError()
		addErr("prepare CF verification", externalErr)
		externalCheckUnavailable = externalErr != nil
	}

	bindingDeleteErr := callCleanup(ctx, ops.DeleteBinding)
	addErr("request binding deletion", bindingDeleteErr)
	bindingGone, bindingWaitErr := waitForCleanupAbsence(ctx, pollInterval, ops.BindingExists)
	addErr("wait for binding finalization", bindingWaitErr)

	cfVerified := true
	if bindingGone {
		if externalCheckUnavailable {
			// The verification limitation is reported above. Provider-managed
			// deletion/finalization still permits safe owned Kubernetes cleanup.
		} else if ops.BindingExistsInCF == nil {
			if len(guids) != 0 {
				cfVerified = false
				addErr("verify CF binding deletion", errors.New("CF binding read operation is not configured"))
			}
		} else {
			for _, guid := range uniqueNonEmpty(guids) {
				gone, err := waitForCFBindingAbsence(ctx, pollInterval, guid, ops.BindingExistsInCF)
				if err != nil {
					cfVerified = false
					addErr("verify CF binding deletion", fmt.Errorf("binding identity %q: %w", guid, err))
					break
				}
				if !gone {
					cfVerified = false
				}
			}
		}
	} else {
		cfVerified = false
	}

	// Remove only the configured output Secret after its managed binding has
	// finalized. It is namespaced, unlike the cluster-scoped managed resources.
	if bindingGone && cfVerified && ctx.Err() == nil {
		addErr("request connection Secret deletion", callCleanup(ctx, ops.DeleteSecret))
		_, err := waitForCleanupAbsence(ctx, pollInterval, ops.SecretExists)
		addErr("wait for connection Secret deletion", err)
	}

	// Do not remove the parent instance or imported wrappers while the binding
	// CR is still finalizing or a configured CF check confirms a key remains.
	if bindingGone && cfVerified && ctx.Err() == nil {
		addErr("request instance deletion", callCleanup(ctx, ops.DeleteInstance))
		instanceGone, err := waitForCleanupAbsence(ctx, pollInterval, ops.InstanceExists)
		if err != nil && instanceGUID != "" {
			err = fmt.Errorf("instance identity %q: %w", instanceGUID, err)
		}
		addErr("wait for instance finalization", err)
		if instanceGone && ctx.Err() == nil {
			addErr("request imported-wrapper deletion", callCleanup(ctx, ops.DeleteImports))
			_, err = waitForCleanupAbsence(ctx, pollInterval, ops.ImportsExist)
			addErr("wait for imported-wrapper deletion", err)
		}
	}

	if err := ctx.Err(); err != nil {
		addErr("cleanup deadline", err)
	}
	return errors.Join(errs...)
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
	for {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		present, err := exists(ctx)
		if err != nil {
			return false, err
		}
		if !present {
			return true, nil
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			stopCleanupTimer(timer)
			return false, ctx.Err()
		case <-timer.C:
		}
	}
}

func waitForCFBindingAbsence(ctx context.Context, interval time.Duration, guid string, exists func(context.Context, string) (bool, error)) (bool, error) {
	for {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		present, err := exists(ctx, guid)
		if err != nil {
			return false, err
		}
		if !present {
			return true, nil
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			stopCleanupTimer(timer)
			return false, ctx.Err()
		case <-timer.C:
		}
	}
}

func stopCleanupTimer(timer *time.Timer) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
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
		return true, errors.New("Cloud Foundry returned no binding without a not-found response")
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
