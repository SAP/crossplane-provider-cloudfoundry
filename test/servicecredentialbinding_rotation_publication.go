package test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServiceCredentialBindingPublicationSnapshot is a single read-back attempt.
// ExpectedDetails must be non-nil when retrieval succeeded; an empty map is a
// successful empty response and is distinct from a retrieval failure.
type ServiceCredentialBindingPublicationSnapshot struct {
	BindingGUID     string
	SecretNamespace string
	SecretName      string
	ExpectedDetails map[string][]byte
	Secret          *corev1.Secret
	ExpectedOwner   *corev1.Secret
}

// WaitForServiceCredentialBindingPublication polls readback until the requested
// binding generation's current credentials are present in its configured Secret.
// Credential values and underlying errors are deliberately excluded from errors.
func WaitForServiceCredentialBindingPublication(
	ctx context.Context,
	timeout, interval time.Duration,
	targetGUID string,
	requireNonEmpty bool,
	read func(context.Context) (ServiceCredentialBindingPublicationSnapshot, error),
) error {
	if err := validatePublicationWait(targetGUID, timeout, read); err != nil {
		return err
	}
	if interval <= 0 {
		interval = time.Second
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return pollForPublication(waitCtx, interval, targetGUID, requireNonEmpty, read)
}

func validatePublicationWait(targetGUID string, timeout time.Duration, read func(context.Context) (ServiceCredentialBindingPublicationSnapshot, error)) error {
	if targetGUID == "" {
		return errors.New("target binding GUID is required")
	}
	if timeout <= 0 {
		return errors.New("publication wait timeout must be positive")
	}
	if read == nil {
		return errors.New("publication read operation is required")
	}
	return nil
}

func pollForPublication(ctx context.Context, interval time.Duration, targetGUID string, requireNonEmpty bool, read func(context.Context) (ServiceCredentialBindingPublicationSnapshot, error)) error {
	for {
		if err := ctx.Err(); err != nil {
			return publicationTimeout(targetGUID, err)
		}
		snapshot, err := read(ctx)
		if err == nil && serviceCredentialBindingPublicationMatches(snapshot, targetGUID, requireNonEmpty) {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return publicationTimeout(targetGUID, err)
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			stopPublicationTimer(timer)
			return publicationTimeout(targetGUID, ctx.Err())
		case <-timer.C:
		}
	}
}

func publicationTimeout(targetGUID string, err error) error {
	return fmt.Errorf("timed out waiting for published credentials for binding %q: %w", targetGUID, err)
}

func serviceCredentialBindingPublicationMatches(snapshot ServiceCredentialBindingPublicationSnapshot, targetGUID string, requireNonEmpty bool) bool {
	if !publicationGenerationMatches(snapshot, targetGUID, requireNonEmpty) {
		return false
	}
	if !publicationSecretMatches(snapshot) {
		return false
	}
	return publicationContainsExpectedDetails(snapshot.Secret.Data, snapshot.ExpectedDetails)
}

func publicationGenerationMatches(snapshot ServiceCredentialBindingPublicationSnapshot, targetGUID string, requireNonEmpty bool) bool {
	return snapshot.BindingGUID == targetGUID && snapshot.ExpectedDetails != nil &&
		(!requireNonEmpty || len(snapshot.ExpectedDetails) != 0)
}

func publicationSecretMatches(snapshot ServiceCredentialBindingPublicationSnapshot) bool {
	secret, expected := snapshot.Secret, snapshot.ExpectedOwner
	return secret != nil && expected != nil && snapshot.SecretNamespace != "" && snapshot.SecretName != "" &&
		secret.Namespace == snapshot.SecretNamespace && secret.Name == snapshot.SecretName &&
		secret.Type == resource.SecretTypeConnection && expected.Namespace == snapshot.SecretNamespace &&
		expected.Name == snapshot.SecretName && sameRotationSecretController(secret, expected)
}

func publicationContainsExpectedDetails(data map[string][]byte, details map[string][]byte) bool {
	for key, value := range details {
		actual, ok := data[key]
		if !ok || !bytes.Equal(actual, value) {
			return false
		}
	}
	return true
}

func sameRotationSecretController(actual, expected *corev1.Secret) bool {
	if actual == nil || expected == nil {
		return false
	}
	a, e := metav1.GetControllerOf(actual), metav1.GetControllerOf(expected)
	return a != nil && e != nil && a.APIVersion == e.APIVersion && a.Kind == e.Kind && a.Name == e.Name && a.UID == e.UID
}

func stopPublicationTimer(timer *time.Timer) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}
