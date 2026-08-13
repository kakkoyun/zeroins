package kubectlobi

import (
	"context"
	"fmt"
	"time"
)

// runBoundedAttach attaches, then blocks for duration, printing remaining time
// to stderr. When the timer fires or the parent context is cancelled (signal),
// it detaches using a fresh context derived from the parent via
// context.WithoutCancel so the signal-triggered detach does not fail instantly.
//
// On cleanup failure the command returns an error naming the orphaned release
// or deployment and pointing at `zeroins sessions reap`.
func runBoundedAttach(ctx context.Context, deps Dependencies, duration time.Duration, detachFn func(context.Context) error, resourceDesc string) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	fmt.Fprintf(deps.Stderr, "Attached. Detach in %s (or press Ctrl+C).\n", duration.Round(time.Second))

	start := time.Now()
	for {
		remaining := duration - time.Since(start)
		if remaining <= 0 {
			break
		}
		select {
		case <-ctx.Done():
			fmt.Fprintf(deps.Stderr, "\nSignal received, detaching...\n")
			return runDetachWithCleanup(ctx, deps, detachFn, resourceDesc)
		case <-timer.C:
			fmt.Fprintf(deps.Stderr, "\nDuration expired, detaching...\n")
			return runDetachWithCleanup(ctx, deps, detachFn, resourceDesc)
		case <-ticker.C:
			fmt.Fprintf(deps.Stderr, "  remaining: %s\n", remaining.Round(time.Second))
		}
	}
	fmt.Fprintf(deps.Stderr, "\nDuration expired, detaching...\n")
	return runDetachWithCleanup(ctx, deps, detachFn, resourceDesc)
}

// runDetachWithCleanup detaches using a fresh context that is not cancelled
// by the parent's signal handler. The parent context is already cancelled
// when a signal fires, so reusing it would make detach fail instantly and
// orphan a privileged DaemonSet.
func runDetachWithCleanup(parentCtx context.Context, deps Dependencies, detachFn func(context.Context) error, resourceDesc string) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(parentCtx), 2*time.Minute)
	defer cancel()

	if err := detachFn(cleanupCtx); err != nil {
		return fmt.Errorf("bounded attach: detach failed, orphaned %s: %w\nRun `zeroins sessions reap` to clean up", resourceDesc, err)
	}
	fmt.Fprintf(deps.Stderr, "Detached successfully.\n")
	return nil
}
