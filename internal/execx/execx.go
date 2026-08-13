// Package execx runs external commands with context cancellation.
package execx

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Runner executes a command and returns its output.
type Runner interface {
	// Run executes name with args and returns combined output (stdout and stderr
	// interleaved). Failures include command output in the error message.
	Run(ctx context.Context, name string, args ...string) (string, error)
	// Output executes name with args and returns stdout only. stderr is excluded
	// from the returned string so JSON parsing is not corrupted by warnings, but
	// failures still include stderr in the error message.
	Output(ctx context.Context, name string, args ...string) (string, error)
}

// OSRunner executes commands on the host.
type OSRunner struct{}

// Run executes name with args and returns combined output. Failures include
// command output in the error message.
func (OSRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(out))
		if message == "" {
			return "", fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
		}
		return "", fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, message)
	}
	return string(out), nil
}

// Output executes name with args and returns stdout only. stderr is excluded
// from the returned string but included in failure error messages.
func (OSRunner) Output(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			return "", fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
		}
		return "", fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, message)
	}
	return string(out), nil
}

// CurrentNamespace returns the namespace selected by the current kubectl context.
func CurrentNamespace(ctx context.Context, runner Runner) (string, error) {
	out, err := runner.Output(ctx, "kubectl", "config", "view", "--minify", "--output", "jsonpath={..namespace}")
	if err != nil {
		return "", fmt.Errorf("read current kubectl namespace: %w", err)
	}
	if namespace := strings.TrimSpace(out); namespace != "" {
		return namespace, nil
	}
	return "default", nil
}

// CurrentContext returns the current kubectl context name (read-only).
func CurrentContext(runner Runner) (string, error) {
	out, err := runner.Output(context.Background(), "kubectl", "config", "current-context")
	if err != nil {
		return "", fmt.Errorf("read current kubectl context: %w", err)
	}
	return strings.TrimSpace(out), nil
}
