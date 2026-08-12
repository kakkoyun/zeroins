package kubectlobi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
)

type runnerCall struct {
	name string
	args []string
}

type runnerResponse struct {
	output string
	err    error
}

type fakeRunner struct {
	calls     []runnerCall
	responses []runnerResponse
	inspect   func(name string, args []string) error
}

func (runner *fakeRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	runner.calls = append(runner.calls, runnerCall{name: name, args: append([]string(nil), args...)})
	if runner.inspect != nil {
		if err := runner.inspect(name, args); err != nil {
			return "", err
		}
	}
	if len(runner.responses) == 0 {
		return "", nil
	}
	response := runner.responses[0]
	runner.responses = runner.responses[1:]
	return response.output, response.err
}

func testDeps(runner *fakeRunner) (Dependencies, *bytes.Buffer, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer
	return Dependencies{
		Runner: runner,
		Client: http.DefaultClient,
		Stdout: &stdout,
		Stderr: &stderr,
		Getenv: func(string) string { return "" },
	}, &stdout, &stderr
}

func TestEndpointValidation(t *testing.T) {
	tests := []struct {
		endpoint string
		wantErr  string
	}{
		{endpoint: "http://collector:4318"},
		{endpoint: "https://collector.example/tenant"},
		{endpoint: "https://collector.example/v1/traces", wantErr: "base URL"},
		{endpoint: "https://collector.example/tenant/v1/metrics/", wantErr: "base URL"},
		{endpoint: "collector:4317", wantErr: "absolute http or https"},
		{endpoint: "ftp://collector", wantErr: "absolute http or https"},
		{endpoint: (&url.URL{Scheme: "https", Host: "collector", User: url.User("example")}).String(), wantErr: "user information"},
		{endpoint: "https://collector?token=secret", wantErr: "query or fragment"},
	}
	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			err := validateOTLPEndpoint(test.endpoint)
			if test.wantErr == "" && err != nil {
				t.Fatalf("validateOTLPEndpoint() error = %v", err)
			}
			if test.wantErr != "" && (err == nil || !strings.Contains(err.Error(), test.wantErr)) {
				t.Fatalf("validateOTLPEndpoint() error = %v, want %q", err, test.wantErr)
			}
		})
	}
}

func TestDaemonSetValuesUseCompleteSignalEndpoints(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		metrics  string
		traces   string
	}{
		{
			name:     "standard HTTP port",
			endpoint: "https://collector.example:4318",
			metrics:  "https://collector.example:4318/v1/metrics",
			traces:   "https://collector.example:4318/v1/traces",
		},
		{
			name:     "gRPC port with base path",
			endpoint: "http://collector.example:4317/tenant",
			metrics:  "http://collector.example:4317/tenant/v1/metrics",
			traces:   "http://collector.example:4317/tenant/v1/traces",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			values, err := daemonSetValues(test.endpoint)
			if err != nil {
				t.Fatalf("daemonSetValues() error = %v", err)
			}
			var document struct {
				Env map[string]string `json:"env"`
			}
			if err := json.Unmarshal(values, &document); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}
			want := map[string]string{
				"OTEL_EXPORTER_OTLP_ENDPOINT":         test.endpoint,
				"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT": test.metrics,
				"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT":  test.traces,
			}
			for key, wantValue := range want {
				if got := document.Env[key]; got != wantValue {
					t.Errorf("%s = %q, want %q", key, got, wantValue)
				}
			}
		})
	}
}

func TestAttachDaemonSetCommand(t *testing.T) {
	var valuesPath string
	runner := &fakeRunner{
		responses: []runnerResponse{{}, {}, {output: "installed\n"}, {output: "obi-agent"}, {output: "ready\n"}},
		inspect: func(name string, args []string) error {
			if name != "helm" || len(args) == 0 || args[0] != "upgrade" {
				return nil
			}
			for index, arg := range args {
				if arg == "--values" && index+1 < len(args) {
					valuesPath = args[index+1]
				}
			}
			info, err := os.Stat(valuesPath)
			if err != nil {
				return err
			}
			if info.Mode().Perm() != 0o600 {
				return fmt.Errorf("temporary values mode = %o", info.Mode().Perm())
			}
			data, err := os.ReadFile(valuesPath)
			if err != nil {
				return err
			}
			for _, want := range []string{
				"OTEL_EXPORTER_OTLP_ENDPOINT",
				"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT",
				"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
				"https://collector:4318",
			} {
				if !strings.Contains(string(data), want) {
					return fmt.Errorf("values missing %q", want)
				}
			}
			return nil
		},
	}
	deps, stdout, stderr := testDeps(runner)
	deps.TempDir = t.TempDir()
	code := Main(context.Background(), []string{"attach", "--endpoint", "https://collector:4318"}, deps)
	if code != 0 {
		t.Fatalf("Main() = %d\nstderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "chart 0.10.0") {
		t.Fatalf("stdout missing chart pin: %s", stdout.String())
	}
	if len(runner.calls) != 5 {
		t.Fatalf("calls = %d, want 5: %#v", len(runner.calls), runner.calls)
	}
	install := runner.calls[2]
	joined := strings.Join(install.args, " ")
	for _, want := range []string{"--version 0.10.0", "--namespace obi-system", "--values"} {
		if !strings.Contains(joined, want) {
			t.Errorf("Helm args missing %q: %s", want, joined)
		}
	}
	if _, err := os.Stat(valuesPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary values file still exists: %v", err)
	}
}

func TestAttachArgumentValidation(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing endpoint", args: []string{"attach"}, want: "required flag"},
		{name: "daemonset deployment", args: []string{"attach", "web", "--endpoint", "http://collector:4318"}, want: "does not accept"},
		{name: "sidecar missing deployment", args: []string{"attach", "--mode", "sidecar", "--endpoint", "http://collector:4318"}, want: "exactly one"},
		{name: "sidecar too many", args: []string{"attach", "a", "b", "--mode", "sidecar", "--endpoint", "http://collector:4318"}, want: "exactly one"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			deps, _, stderr := testDeps(&fakeRunner{})
			if code := Main(context.Background(), test.args, deps); code != 1 {
				t.Fatalf("Main() = %d, want 1", code)
			}
			if !strings.Contains(stderr.String(), test.want) {
				t.Fatalf("stderr missing %q: %s", test.want, stderr.String())
			}
		})
	}
}

func TestStatusPropagatesKubectlError(t *testing.T) {
	runner := &fakeRunner{responses: []runnerResponse{{err: errors.New("authentication required")}}}
	deps, _, stderr := testDeps(runner)
	if code := Main(context.Background(), []string{"status"}, deps); code != 1 {
		t.Fatalf("Main() = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "authentication required") {
		t.Fatalf("stderr lost kubectl error: %s", stderr.String())
	}
	if got := strings.Join(runner.calls[0].args, " "); !strings.Contains(got, "-n obi-system") {
		t.Fatalf("status namespace args = %q", got)
	}
}

func deploymentJSON(managed bool, share *bool, withOBI bool, original string) string {
	annotations := map[string]string{}
	if managed {
		annotations[managedAnnotation] = "true"
		annotations[originalShareAnnotation] = original
	}
	containers := []map[string]string{{"name": "app"}}
	if withOBI {
		containers = append(containers, map[string]string{"name": "obi"})
	}
	spec := map[string]any{"containers": containers}
	if share != nil {
		spec["shareProcessNamespace"] = *share
	}
	document := map[string]any{"spec": map[string]any{"template": map[string]any{"metadata": map[string]any{"annotations": annotations}, "spec": spec}}}
	data, _ := json.Marshal(document)
	return string(data)
}

func TestAttachSidecarRecordsStateAndIsIdempotent(t *testing.T) {
	falseValue := false
	runner := &fakeRunner{responses: []runnerResponse{{output: deploymentJSON(false, &falseValue, false, "")}, {output: "patched\n"}, {}, {}}}
	deps, _, _ := testDeps(runner)
	if err := attachSidecar(context.Background(), deps, "web", "prod", "https://collector:4318"); err != nil {
		t.Fatalf("attachSidecar() error = %v", err)
	}
	if len(runner.calls) != 4 {
		t.Fatalf("calls = %d, want 4", len(runner.calls))
	}
	patch := runner.calls[1].args[len(runner.calls[1].args)-1]
	for _, want := range []string{managedAnnotation, originalShareAnnotation, `"false"`, obiImage, "OTEL_EBPF_AUTO_TARGET_EXE", "OTEL_EXPORTER_OTLP_ENDPOINT"} {
		if !strings.Contains(patch, want) {
			t.Errorf("patch missing %q: %s", want, patch)
		}
	}

	idempotentRunner := &fakeRunner{responses: []runnerResponse{{output: deploymentJSON(true, &falseValue, true, "false")}}}
	idempotentDeps, stdout, _ := testDeps(idempotentRunner)
	if err := attachSidecar(context.Background(), idempotentDeps, "web", "prod", "https://collector:4318"); err != nil {
		t.Fatalf("idempotent attach error = %v", err)
	}
	if len(idempotentRunner.calls) != 1 || !strings.Contains(stdout.String(), "already has") {
		t.Fatalf("idempotent attach made unexpected calls: %#v; stdout=%s", idempotentRunner.calls, stdout.String())
	}
}

func TestAttachSidecarUsesCurrentNamespace(t *testing.T) {
	runner := &fakeRunner{responses: []runnerResponse{{output: "team-a"}, {output: deploymentJSON(false, nil, false, "")}, {}, {}, {}}}
	deps, _, stderr := testDeps(runner)
	code := Main(context.Background(), []string{"attach", "web", "--mode", "sidecar", "--endpoint", "http://collector:4318"}, deps)
	if code != 0 {
		t.Fatalf("Main() = %d: %s", code, stderr.String())
	}
	if got := strings.Join(runner.calls[1].args, " "); !strings.Contains(got, "-n team-a") {
		t.Fatalf("deployment lookup args = %q", got)
	}
}

func TestDetachSidecarRestoresOriginalShareState(t *testing.T) {
	tests := []struct {
		name     string
		share    *bool
		original string
		wantJSON string
	}{
		{name: "unset", original: "unset", wantJSON: `"shareProcessNamespace":null`},
		{name: "false", share: boolPointer(true), original: "false", wantJSON: `"shareProcessNamespace":false`},
		{name: "true", share: boolPointer(true), original: "true", wantJSON: `"shareProcessNamespace":true`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner := &fakeRunner{responses: []runnerResponse{{output: deploymentJSON(true, test.share, true, test.original)}, {}, {}, {}}}
			deps, _, _ := testDeps(runner)
			if err := detachSidecar(context.Background(), deps, "web", "prod"); err != nil {
				t.Fatalf("detachSidecar() error = %v", err)
			}
			patch := runner.calls[1].args[len(runner.calls[1].args)-1]
			if !strings.Contains(patch, test.wantJSON) || !strings.Contains(patch, `"$patch":"delete"`) {
				t.Fatalf("patch does not restore %s: %s", test.original, patch)
			}
		})
	}
}

func TestSidecarOwnershipRefusal(t *testing.T) {
	runner := &fakeRunner{responses: []runnerResponse{{output: deploymentJSON(false, nil, true, "")}}}
	deps, _, _ := testDeps(runner)
	if err := attachSidecar(context.Background(), deps, "web", "prod", "http://collector:4318"); err == nil || !strings.Contains(err.Error(), "unmanaged") {
		t.Fatalf("attachSidecar() error = %v, want unmanaged refusal", err)
	}

	runner = &fakeRunner{responses: []runnerResponse{{output: deploymentJSON(false, nil, true, "")}}}
	deps, _, _ = testDeps(runner)
	if err := detachSidecar(context.Background(), deps, "web", "prod"); err == nil || !strings.Contains(err.Error(), "unmanaged") {
		t.Fatalf("detachSidecar() error = %v, want unmanaged refusal", err)
	}
}

func boolPointer(value bool) *bool { return &value }

func TestHelpAndVersion(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"version"}} {
		deps, stdout, stderr := testDeps(&fakeRunner{})
		if code := Main(context.Background(), args, deps); code != 0 {
			t.Fatalf("Main(%v) = %d: %s", args, code, stderr.String())
		}
		if stdout.Len() == 0 {
			t.Fatalf("Main(%v) produced no output", args)
		}
	}
}

func TestRunnerErrorContext(t *testing.T) {
	runner := &fakeRunner{responses: []runnerResponse{{err: fmt.Errorf("cluster unreachable")}}}
	deps, _, _ := testDeps(runner)
	err := detachDaemonSet(context.Background(), deps, "obi-system")
	if err == nil || !strings.Contains(err.Error(), "cluster unreachable") {
		t.Fatalf("detachDaemonSet() error = %v", err)
	}
}
