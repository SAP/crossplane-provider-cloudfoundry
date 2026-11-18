package upgrade

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

type cleanupContextKey struct{}

func TestUpgradeCleanupNoHooksPreservesDefaultDeletion(t *testing.T) {
	var calls []string
	err := runResourceCleanup(context.Background(), time.Second, nil, func(context.Context) error {
		calls = append(calls, "delete")
		return nil
	}, nil)

	if err != nil {
		t.Fatalf("runResourceCleanup() error = %v", err)
	}
	if !reflect.DeepEqual(calls, []string{"delete"}) {
		t.Fatalf("cleanup calls = %v, want [delete]", calls)
	}
}

func TestUpgradeCleanupOrderAndContinuesAfterErrors(t *testing.T) {
	preErr := errors.New("preparation failed")
	deleteErr := errors.New("resource deletion failed")
	postErr := errors.New("verification failed")
	var calls []string

	err := runResourceCleanup(context.Background(), time.Second,
		func(context.Context) error {
			calls = append(calls, "prepare")
			return preErr
		},
		func(context.Context) error {
			calls = append(calls, "delete")
			return deleteErr
		},
		func(context.Context) error {
			calls = append(calls, "verify")
			return postErr
		},
	)

	if !reflect.DeepEqual(calls, []string{"prepare", "delete", "verify"}) {
		t.Fatalf("cleanup calls = %v, want [prepare delete verify]", calls)
	}
	for name, want := range map[string]error{
		"preparation":  preErr,
		"deletion":     deleteErr,
		"verification": postErr,
	} {
		if !errors.Is(err, want) {
			t.Errorf("joined cleanup error does not retain %s error: %v", name, err)
		}
	}
}

func TestUpgradeCleanupDetachesCanceledContextAndPreservesValues(t *testing.T) {
	parent, cancel := context.WithCancel(context.WithValue(context.Background(), cleanupContextKey{}, "scenario"))
	cancel()
	var calls int
	var cleanupDeadline time.Time

	err := runResourceCleanup(parent, time.Second,
		func(ctx context.Context) error { return checkCleanupContext(ctx, &calls, &cleanupDeadline) },
		func(ctx context.Context) error { return checkCleanupContext(ctx, &calls, &cleanupDeadline) },
		func(ctx context.Context) error { return checkCleanupContext(ctx, &calls, &cleanupDeadline) },
	)

	if err != nil {
		t.Fatalf("runResourceCleanup() error = %v", err)
	}
	if calls != 3 {
		t.Fatalf("cleanup calls = %d, want 3", calls)
	}
}

func checkCleanupContext(ctx context.Context, calls *int, cleanupDeadline *time.Time) error {
	(*calls)++
	if got := ctx.Value(cleanupContextKey{}); got != "scenario" {
		return errors.New("cleanup context lost scenario value")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	deadline, ok := ctx.Deadline()
	if !ok || !deadline.After(time.Now()) || time.Until(deadline) > time.Second {
		return errors.New("cleanup context does not have a usable one-second deadline")
	}
	if cleanupDeadline.IsZero() {
		*cleanupDeadline = deadline
	} else if !deadline.Equal(*cleanupDeadline) {
		return errors.New("cleanup phases do not share one deadline")
	}
	return nil
}

func TestUpgradeCleanupDeadlineIsObservableAndLaterPhasesAreAttempted(t *testing.T) {
	var calls []string
	err := runResourceCleanup(context.Background(), time.Millisecond,
		func(ctx context.Context) error {
			calls = append(calls, "prepare")
			<-ctx.Done()
			return ctx.Err()
		},
		func(ctx context.Context) error {
			calls = append(calls, "delete")
			return ctx.Err()
		},
		func(ctx context.Context) error {
			calls = append(calls, "verify")
			return ctx.Err()
		},
	)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("runResourceCleanup() error = %v, want deadline exceeded", err)
	}
	if !reflect.DeepEqual(calls, []string{"prepare", "delete", "verify"}) {
		t.Fatalf("cleanup calls = %v, want all phases attempted", calls)
	}
}
