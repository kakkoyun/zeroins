package zeroins

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

type zeroinsCall struct {
	name   string
	args   []string
	output bool
}

type zeroinsResponse struct {
	output string
	err    error
}

type zeroinsRunner struct {
	calls     []zeroinsCall
	responses []zeroinsResponse
}

func (r *zeroinsRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	r.calls = append(r.calls, zeroinsCall{name: name, args: append([]string(nil), args...), output: false})
	return r.respond()
}

func (r *zeroinsRunner) Output(_ context.Context, name string, args ...string) (string, error) {
	r.calls = append(r.calls, zeroinsCall{name: name, args: append([]string(nil), args...), output: true})
	return r.respond()
}

func (r *zeroinsRunner) respond() (string, error) {
	if len(r.responses) == 0 {
		return "", nil
	}
	resp := r.responses[0]
	r.responses = r.responses[1:]
	return resp.output, resp.err
}

func zeroinsTestDeps(runner *zeroinsRunner) (Dependencies, *bytes.Buffer, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer
	return Dependencies{
		Runner: runner,
		Stdout: &stdout,
		Stderr: &stderr,
		Getenv: func(string) string { return "" },
	}, &stdout, &stderr
}

func TestZeroinsHelp(t *testing.T) {
	deps, stdout, _ := zeroinsTestDeps(&zeroinsRunner{})
	if code := Main(context.Background(), []string{"--help"}, deps); code != 0 {
		t.Fatal("help should exit 0")
	}
	for _, want := range []string{"doctor", "obi", "profiler", "otelc", "sessions", "version"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("help missing %q: %s", want, stdout.String())
		}
	}
}

func TestZeroinsVersionJSON(t *testing.T) {
	deps, stdout, _ := zeroinsTestDeps(&zeroinsRunner{})
	if code := Main(context.Background(), []string{"version", "-o", "json"}, deps); code != 0 {
		t.Fatalf("version -o json = %d", code)
	}
	if !strings.Contains(stdout.String(), `"tool"`) || !strings.Contains(stdout.String(), `zeroins`) {
		t.Fatalf("version json = %s", stdout.String())
	}
}

func TestZeroinsVersionTable(t *testing.T) {
	deps, stdout, _ := zeroinsTestDeps(&zeroinsRunner{})
	if code := Main(context.Background(), []string{"version"}, deps); code != 0 {
		t.Fatalf("version = %d", code)
	}
	for _, want := range []string{"zeroins:", "OBI:", "otelc:", "profiler:"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("version table missing %q: %s", want, stdout.String())
		}
	}
}

func TestZeroinsObiLookupDefaultTable(t *testing.T) {
	deps, stdout, _ := zeroinsTestDeps(&zeroinsRunner{})
	if code := Main(context.Background(), []string{"obi", "lookup", "net/http"}, deps); code != 0 {
		t.Fatalf("obi lookup = %d", code)
	}
	if !strings.Contains(stdout.String(), "LIBRARY") || !strings.Contains(stdout.String(), "net/http") {
		t.Fatalf("obi lookup table = %s", stdout.String())
	}
}

func TestZeroinsOtelcLookupJSON(t *testing.T) {
	deps, stdout, _ := zeroinsTestDeps(&zeroinsRunner{})
	if code := Main(context.Background(), []string{"otelc", "lookup", "net/http", "-o", "json"}, deps); code != 0 {
		t.Fatalf("otelc lookup json = %d", code)
	}
	if !strings.Contains(stdout.String(), `otelc`) || !strings.Contains(stdout.String(), `net/http`) {
		t.Fatalf("otelc lookup json = %s", stdout.String())
	}
}

func TestZeroinsObiLookupMarkdownReproducesOldBytes(t *testing.T) {
	deps, stdout, _ := zeroinsTestDeps(&zeroinsRunner{})
	if code := Main(context.Background(), []string{"obi", "lookup", "net/http", "-o", "markdown"}, deps); code != 0 {
		t.Fatalf("obi lookup markdown = %d", code)
	}
	if !strings.HasPrefix(stdout.String(), "# OBI v0.10.0") {
		t.Fatalf("obi lookup markdown = %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "| `net/http` | `>= 1.17` |") {
		t.Errorf("markdown missing table row: %s", stdout.String())
	}
}

// isMutating returns true if the runner call would mutate the cluster.
func isMutating(call zeroinsCall) bool {
	if call.name == "helm" {
		for _, a := range call.args {
			if a == "upgrade" || a == "install" || a == "uninstall" {
				return true
			}
		}
	}
	if call.name == "kubectl" {
		for _, a := range call.args {
			if a == "patch" || a == "label" || a == "annotate" || a == "apply" || a == "delete" || a == "rollout" {
				return true
			}
		}
	}
	return false
}

func TestZeroinsObiAttachDryRunNoMutatingCalls(t *testing.T) {
	runner := &zeroinsRunner{}
	deps, stdout, _ := zeroinsTestDeps(runner)
	if code := Main(context.Background(), []string{"obi", "attach", "--dry-run", "--endpoint", "https://collector:4318"}, deps); code != 0 {
		t.Fatalf("obi attach --dry-run = %d", code)
	}
	for _, c := range runner.calls {
		if isMutating(c) {
			t.Errorf("dry-run made mutating call: %s %s", c.name, strings.Join(c.args, " "))
		}
	}
	if !strings.Contains(stdout.String(), "Plan") || !strings.Contains(stdout.String(), "daemonset") {
		t.Fatalf("dry-run output missing plan: %s", stdout.String())
	}
}

func TestZeroinsObiAttachDryRunJSON(t *testing.T) {
	runner := &zeroinsRunner{}
	deps, stdout, _ := zeroinsTestDeps(runner)
	if code := Main(context.Background(), []string{"obi", "attach", "--dry-run", "-o", "json", "--endpoint", "https://collector:4318"}, deps); code != 0 {
		t.Fatalf("obi attach --dry-run -o json = %d", code)
	}
	if !strings.Contains(stdout.String(), `daemonset`) || !strings.Contains(stdout.String(), `commands`) {
		t.Fatalf("dry-run json missing key fields: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), `values`) {
		t.Errorf("dry-run json missing values: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), `privilege`) {
		t.Errorf("dry-run json missing privilege: %s", stdout.String())
	}
}

func TestZeroinsProfilerAttachDryRun(t *testing.T) {
	runner := &zeroinsRunner{}
	deps, stdout, _ := zeroinsTestDeps(runner)
	if code := Main(context.Background(), []string{"profiler", "attach", "--dry-run", "--endpoint", "profiles.example:4317"}, deps); code != 0 {
		t.Fatalf("profiler attach --dry-run = %d", code)
	}
	for _, c := range runner.calls {
		if isMutating(c) {
			t.Errorf("dry-run made mutating call: %s %s", c.name, strings.Join(c.args, " "))
		}
	}
	if !strings.Contains(stdout.String(), "profiler") || !strings.Contains(stdout.String(), "tracefs") {
		t.Fatalf("dry-run output missing privilege: %s", stdout.String())
	}
}

func TestZeroinsObiValuesNoClusterAccess(t *testing.T) {
	runner := &zeroinsRunner{}
	deps, stdout, _ := zeroinsTestDeps(runner)
	if code := Main(context.Background(), []string{"obi", "values", "--endpoint", "https://collector:4318"}, deps); code != 0 {
		t.Fatalf("obi values = %d", code)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("values made %d runner calls, want 0", len(runner.calls))
	}
	if !strings.Contains(stdout.String(), "OTEL_EXPORTER_OTLP_ENDPOINT") {
		t.Fatalf("values output missing endpoint env: %s", stdout.String())
	}
}

func TestZeroinsProfilerValuesNoClusterAccess(t *testing.T) {
	runner := &zeroinsRunner{}
	deps, stdout, _ := zeroinsTestDeps(runner)
	if code := Main(context.Background(), []string{"profiler", "values", "--endpoint", "profiles.example:4317"}, deps); code != 0 {
		t.Fatalf("profiler values = %d", code)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("values made %d runner calls, want 0", len(runner.calls))
	}
	if !strings.Contains(stdout.String(), "profiles.example:4317") {
		t.Fatalf("values output missing endpoint: %s", stdout.String())
	}
}
