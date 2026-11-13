package upgrade

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type cleanupAction func(context.Context) error

// runResourceCleanup runs every resource-cleanup phase with one fresh, bounded
// context. It detaches cancellation from the assessment context while
// preserving its values, so a failed assessment does not prevent cleanup.
// Errors are joined after all phases have been attempted.
func runResourceCleanup(
	parent context.Context,
	timeout time.Duration,
	before, removeResources, after cleanupAction,
) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), timeout)
	defer cancel()

	var errs []error
	for _, phase := range []struct {
		name   string
		action cleanupAction
	}{
		{name: "pre-resource cleanup", action: before},
		{name: "resource deletion", action: removeResources},
		{name: "post-resource cleanup", action: after},
	} {
		if phase.action == nil {
			continue
		}
		if err := phase.action(ctx); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", phase.name, err))
		}
	}

	return errors.Join(errs...)
}
