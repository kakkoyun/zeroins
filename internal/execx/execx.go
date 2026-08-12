// Package execx runs external commands with context cancellation.
package execx

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Runner executes a command and returns its combined output.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
}

// OSRunner executes commands on the host.
type OSRunner struct{}

// Run executes name with args and includes command output in failures.
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

// CurrentNamespace returns the namespace selected by the current kubectl context.
func CurrentNamespace(ctx context.Context, runner Runner) (string, error) {
	out, err := runner.Run(ctx, "kubectl", "config", "view", "--minify", "--output", "jsonpath={..namespace}")
	if err != nil {
		return "", fmt.Errorf("read current kubectl namespace: %w", err)
	}
	if namespace := strings.TrimSpace(out); namespace != "" {
		return namespace, nil
	}
	return "default", nil
}
