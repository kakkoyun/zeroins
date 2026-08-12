package execx

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestOSRunnerIncludesOutputInError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell assertion is Unix-specific")
	}
	_, err := (OSRunner{}).Run(context.Background(), "sh", "-c", "printf failure >&2; exit 7")
	if err == nil || !strings.Contains(err.Error(), "failure") {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestOSRunnerHonorsCancellation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sleep assertion is Unix-specific")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := (OSRunner{}).Run(ctx, "sleep", "5")
	if err == nil {
		t.Fatal("Run() returned nil error after cancellation")
	}
	if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("context error = %v", ctx.Err())
	}
}
