package test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
)

// ValidateRotationCredentialChange checks the credential field configured for
// the XSUAA rotation fixtures without exposing either credential in errors.
func ValidateRotationCredentialChange(previous, current map[string]any) error {
	oldSecret, ok := previous["clientsecret"].(string)
	if !ok || oldSecret == "" {
		return errors.New("previous binding credentials have no non-empty clientsecret")
	}
	newSecret, ok := current["clientsecret"].(string)
	if !ok || newSecret == "" {
		return errors.New("current binding credentials have no non-empty clientsecret")
	}
	if oldSecret == newSecret {
		return errors.New("binding clientsecret did not change across rotation")
	}
	return nil
}

// RotationActiveBindingUsable reports whether a binding generation is observed,
// succeeded, and present in Cloud Foundry.
func RotationActiveBindingUsable(guid string, operationSucceeded, cfPresent bool) bool {
	return guid != "" && operationSucceeded && cfPresent
}

// RotationRetiredKeyCollected reports completion only when the CF key is absent
// and no non-nil status entry continues to track its GUID.
func RotationRetiredKeyCollected(guid string, cfPresent bool, retired []*v1alpha1.SCBResource) bool {
	if guid == "" || cfPresent {
		return false
	}
	for _, key := range retired {
		if key != nil && key.GUID == guid {
			return false
		}
	}
	return true
}

// WaitForRotationRetiredKeyCollection requires both external absence and status
// garbage collection. Callback errors stop the assessment instead of being
// mistaken for not-found responses.
func WaitForRotationRetiredKeyCollection(ctx context.Context, timeout, interval time.Duration, guid string, readCF func(context.Context) (bool, error), readStatus func(context.Context) ([]*v1alpha1.SCBResource, error)) error {
	if err := validateRotationCollectionWait(guid, timeout, readCF, readStatus); err != nil {
		return err
	}
	if interval <= 0 {
		interval = time.Second
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for {
		if err := waitCtx.Err(); err != nil {
			return fmt.Errorf("timed out waiting for retired-key collection: %w", err)
		}
		collected, err := checkRotationRetiredKeyCollection(waitCtx, guid, readCF, readStatus)
		if err != nil {
			return err
		}
		if collected {
			return nil
		}
		if err := waitRotationCollectionInterval(waitCtx, interval); err != nil {
			return fmt.Errorf("timed out waiting for retired-key collection: %w", err)
		}
	}
}

func validateRotationCollectionWait(guid string, timeout time.Duration, readCF func(context.Context) (bool, error), readStatus func(context.Context) ([]*v1alpha1.SCBResource, error)) error {
	if guid == "" || timeout <= 0 || readCF == nil || readStatus == nil {
		return errors.New("retired-key collection wait requires a GUID, positive timeout, and read callbacks")
	}
	return nil
}

func checkRotationRetiredKeyCollection(ctx context.Context, guid string, readCF func(context.Context) (bool, error), readStatus func(context.Context) ([]*v1alpha1.SCBResource, error)) (bool, error) {
	cfPresent, err := readCF(ctx)
	if err != nil {
		return false, errors.New("cloud foundry retired-key verification failed")
	}
	retired, err := readStatus(ctx)
	if err != nil {
		return false, errors.New("kubernetes retired-key status read failed")
	}
	return RotationRetiredKeyCollected(guid, cfPresent, retired), nil
}

func waitRotationCollectionInterval(ctx context.Context, interval time.Duration) error {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
