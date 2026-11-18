package test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
)

func TestRotationCleanupWaitsForBindingAndExternalDeletionInOrder(t *testing.T) {
	var actions []string
	bindingReads, cfReads := 0, 0
	ops := rotationCleanupOps()
	ops.CaptureBindingGUIDs = func(context.Context) ([]string, error) {
		actions = append(actions, "capture")
		return []string{"k0", "k1", "k0", ""}, nil
	}
	ops.CaptureInstanceGUID = func(context.Context) (string, error) { return "instance-guid", nil }
	ops.DeleteBinding = func(context.Context) error { actions = append(actions, "delete-binding"); return nil }
	ops.BindingExists = func(context.Context) (bool, error) {
		bindingReads++
		actions = append(actions, "check-binding")
		return bindingReads < 3, nil
	}
	ops.BindingExistsInCF = func(_ context.Context, guid string) (bool, error) {
		cfReads++
		actions = append(actions, "check-cf-"+guid)
		return false, nil
	}
	ops.DeleteSecret = func(context.Context) error { actions = append(actions, "delete-secret"); return nil }
	ops.SecretExists = func(context.Context) (bool, error) { return false, nil }
	ops.DeleteInstance = func(context.Context) error { actions = append(actions, "delete-instance"); return nil }
	ops.InstanceExists = func(context.Context) (bool, error) { actions = append(actions, "check-instance"); return false, nil }
	ops.DeleteImports = func(context.Context) error { actions = append(actions, "delete-imports"); return nil }
	ops.ImportsExist = func(context.Context) (bool, error) { actions = append(actions, "check-imports"); return false, nil }

	if err := RunServiceCredentialBindingRotationCleanup(context.Background(), time.Second, time.Millisecond, ops); err != nil {
		t.Fatalf("cleanup returned error: %v", err)
	}
	if bindingReads < 3 || cfReads != 2 {
		t.Fatalf("expected async Kube/CF deletion polling (binding reads %d, CF reads %d)", bindingReads, cfReads)
	}
	if !ordered(actions, "delete-binding", "check-binding", "check-cf-k0", "delete-secret", "delete-instance", "delete-imports") {
		t.Fatalf("cleanup phases ran out of dependency order: %v", actions)
	}
}

func TestRotationCleanupReportsDeleteErrorButContinuesPolling(t *testing.T) {
	var instanceDeletes int
	ops := rotationCleanupOps()
	ops.DeleteBinding = func(context.Context) error { return errors.New("delete response lost") }
	ops.BindingExists = func(context.Context) (bool, error) { return false, nil }
	ops.DeleteInstance = func(context.Context) error { instanceDeletes++; return nil }

	err := RunServiceCredentialBindingRotationCleanup(context.Background(), time.Second, time.Millisecond, ops)
	if err == nil || !strings.Contains(err.Error(), "request binding deletion") {
		t.Fatalf("expected binding deletion error to be reported, got %v", err)
	}
	if instanceDeletes != 1 {
		t.Fatalf("cleanup did not continue after confirming binding absence; instance deletes=%d", instanceDeletes)
	}
}

func TestRotationCleanupRetainsParentsWhileBindingOrCFKeyRemains(t *testing.T) {
	for _, tc := range []struct {
		name           string
		bindingPresent bool
		cfPresent      bool
	}{
		{name: "binding finalizer remains", bindingPresent: true},
		{name: "external key remains", cfPresent: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var instanceDeletes, importDeletes int
			ops := rotationCleanupOps()
			ops.CaptureBindingGUIDs = func(context.Context) ([]string, error) { return []string{"owned-key"}, nil }
			ops.BindingExists = func(context.Context) (bool, error) { return tc.bindingPresent, nil }
			ops.BindingExistsInCF = func(context.Context, string) (bool, error) { return tc.cfPresent, nil }
			ops.DeleteInstance = func(context.Context) error { instanceDeletes++; return nil }
			ops.DeleteImports = func(context.Context) error { importDeletes++; return nil }

			err := RunServiceCredentialBindingRotationCleanup(context.Background(), 15*time.Millisecond, time.Millisecond, ops)
			if err == nil {
				t.Fatal("expected bounded cleanup error while binding/key remains")
			}
			if instanceDeletes != 0 || importDeletes != 0 {
				t.Fatalf("deleted dependency after incomplete binding cleanup (instance=%d imports=%d)", instanceDeletes, importDeletes)
			}
		})
	}
}

func TestRotationCleanupUnavailableCFCheckStillDeletesOwnedKubernetesResources(t *testing.T) {
	var actions []string
	ops := rotationCleanupOps()
	ops.CaptureBindingGUIDs = func(context.Context) ([]string, error) { return []string{"owned-key"}, nil }
	ops.ExternalCheckError = func() error { return errors.New("CF read client unavailable") }
	ops.DeleteBinding = func(context.Context) error { actions = append(actions, "binding-delete"); return nil }
	ops.BindingExists = func(context.Context) (bool, error) { return false, nil }
	ops.DeleteInstance = func(context.Context) error { actions = append(actions, "instance-delete"); return nil }
	ops.InstanceExists = func(context.Context) (bool, error) { return false, nil }
	ops.DeleteImports = func(context.Context) error { actions = append(actions, "imports-delete"); return nil }
	ops.ImportsExist = func(context.Context) (bool, error) { return false, nil }

	err := RunServiceCredentialBindingRotationCleanup(context.Background(), time.Second, time.Millisecond, ops)
	if err == nil || !strings.Contains(err.Error(), "CF read client unavailable") {
		t.Fatalf("expected external verification limitation to be reported, got %v", err)
	}
	if !reflect.DeepEqual(actions, []string{"binding-delete", "instance-delete", "imports-delete"}) {
		t.Fatalf("safe owned Kubernetes cleanup did not continue: %v", actions)
	}
}

func TestRotationCleanupContinuesIndependentWorkAfterPartialSetup(t *testing.T) {
	var actions []string
	ops := rotationCleanupOps()
	ops.CaptureBindingGUIDs = func(context.Context) ([]string, error) { return nil, nil }
	ops.DeleteBinding = func(context.Context) error { actions = append(actions, "binding-delete"); return nil }
	ops.BindingExists = func(context.Context) (bool, error) { return false, nil }
	ops.DeleteInstance = func(context.Context) error { actions = append(actions, "instance-delete"); return nil }
	ops.InstanceExists = func(context.Context) (bool, error) { return false, nil }
	ops.DeleteImports = func(context.Context) error { actions = append(actions, "imports-delete"); return nil }
	ops.ImportsExist = func(context.Context) (bool, error) { return false, nil }

	if err := RunServiceCredentialBindingRotationCleanup(context.Background(), time.Second, time.Millisecond, ops); err != nil {
		t.Fatalf("partial setup cleanup returned error: %v", err)
	}
	if !reflect.DeepEqual(actions, []string{"binding-delete", "instance-delete", "imports-delete"}) {
		t.Fatalf("unexpected cleanup phases: %v", actions)
	}
}

func TestRotationCleanupKubernetesDeleteNotFoundIsBenign(t *testing.T) {
	err := apierrors.NewNotFound(schema.GroupResource{Resource: "servicecredentialbindings"}, "owned")
	if got := IgnoreRotationCleanupNotFound(err); got != nil {
		t.Fatalf("Kubernetes NotFound was not normalized: %v", got)
	}
}

func TestRotationCleanupCFBindingNotFoundVariants(t *testing.T) {
	for _, err := range []error{cfresource.NewResourceNotFoundError(), cfresource.NewServiceBindingNotFoundError()} {
		present, getErr := ServiceCredentialBindingExistsInCF(context.Background(), rotationCFGetter{err: err}, "owned-key")
		if getErr != nil || present {
			t.Fatalf("CF not-found should confirm deletion: present=%t err=%v", present, getErr)
		}
	}
	present, err := ServiceCredentialBindingExistsInCF(context.Background(), rotationCFGetter{binding: &cfresource.ServiceCredentialBinding{GUID: "owned-key"}}, "owned-key")
	if err != nil || !present {
		t.Fatalf("existing CF binding was not retained as present: present=%t err=%v", present, err)
	}
	if _, err := ServiceCredentialBindingExistsInCF(context.Background(), rotationCFGetter{err: errors.New("temporary API failure")}, "owned-key"); err == nil {
		t.Fatal("unexpected CF read error was treated as absence")
	}
}

func TestRotationRetiredKeyCollectionRequiresCFAndStatusAbsence(t *testing.T) {
	for _, tc := range []struct {
		name             string
		guid             string
		cfPresent        bool
		retired          []*v1alpha1.SCBResource
		wantCollected    bool
		wantTrackingLoss bool
	}{
		{name: "both absent", guid: "k0", wantCollected: true},
		{name: "both present while deleting", guid: "k0", cfPresent: true, retired: []*v1alpha1.SCBResource{{GUID: "k0"}}},
		{name: "CF key remains without status tracking", guid: "k0", cfPresent: true, wantTrackingLoss: true},
		{name: "status record remains after CF deletion", guid: "k0", retired: []*v1alpha1.SCBResource{{GUID: "k0"}}},
		{name: "nil status entry ignored", guid: "k0", retired: []*v1alpha1.SCBResource{nil, {GUID: "k1"}}, wantCollected: true},
		{name: "missing identity is rejected", wantTrackingLoss: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ValidateRetiredKeyDeletionObservation(tc.guid, tc.cfPresent, tc.retired)
			if got != tc.wantCollected || (err != nil) != tc.wantTrackingLoss {
				t.Fatalf("retired-key observation = (%t, %v), want collected=%t trackingLoss=%t", got, err, tc.wantCollected, tc.wantTrackingLoss)
			}
			if got := RotationRetiredKeyCollected(tc.guid, tc.cfPresent, tc.retired); got != tc.wantCollected {
				t.Fatalf("retired-key collection = %t, want %t", got, tc.wantCollected)
			}
		})
	}
}

type rotationCFGetter struct {
	binding *cfresource.ServiceCredentialBinding
	err     error
}

func (g rotationCFGetter) Get(context.Context, string) (*cfresource.ServiceCredentialBinding, error) {
	return g.binding, g.err
}

func TestRotationCleanupDetachesAssessmentCancellationAndHonorsBound(t *testing.T) {
	parent, cancel := context.WithCancel(context.WithValue(context.Background(), cleanupContextKey{}, "preserved"))
	cancel()
	var observedValue any
	ops := rotationCleanupOps()
	ops.CaptureBindingGUIDs = func(ctx context.Context) ([]string, error) {
		observedValue = ctx.Value(cleanupContextKey{})
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, nil
	}
	if err := RunServiceCredentialBindingRotationCleanup(parent, time.Second, time.Millisecond, ops); err != nil {
		t.Fatalf("cleanup inherited assessment cancellation: %v", err)
	}
	if observedValue != "preserved" {
		t.Fatalf("cleanup did not preserve context values: %v", observedValue)
	}

	ops = rotationCleanupOps()
	ops.CaptureBindingGUIDs = func(ctx context.Context) ([]string, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	start := time.Now()
	err := RunServiceCredentialBindingRotationCleanup(context.Background(), 10*time.Millisecond, time.Millisecond, ops)
	if err == nil || time.Since(start) > time.Second {
		t.Fatalf("cleanup deadline was not bounded: err=%v elapsed=%s", err, time.Since(start))
	}
}

type cleanupContextKey struct{}

func rotationCleanupOps() ServiceCredentialBindingRotationCleanup {
	return ServiceCredentialBindingRotationCleanup{
		CaptureBindingGUIDs: func(context.Context) ([]string, error) { return nil, nil },
		CaptureInstanceGUID: func(context.Context) (string, error) { return "", nil },
		DeleteBinding:       func(context.Context) error { return nil },
		BindingExists:       func(context.Context) (bool, error) { return false, nil },
		BindingExistsInCF:   func(context.Context, string) (bool, error) { return false, nil },
		DeleteSecret:        func(context.Context) error { return nil },
		SecretExists:        func(context.Context) (bool, error) { return false, nil },
		DeleteInstance:      func(context.Context) error { return nil },
		InstanceExists:      func(context.Context) (bool, error) { return false, nil },
		DeleteImports:       func(context.Context) error { return nil },
		ImportsExist:        func(context.Context) (bool, error) { return false, nil },
	}
}

func ordered(actions []string, sequence ...string) bool {
	at := 0
	for _, action := range actions {
		if action == sequence[at] {
			at++
			if at == len(sequence) {
				return true
			}
		}
	}
	return false
}
