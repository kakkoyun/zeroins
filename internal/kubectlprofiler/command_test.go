package kubectlprofiler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type call struct {
	name string
	args []string
}

type response struct {
	output string
	err    error
}

type fakeRunner struct {
	calls     []call
	responses []response
	inspect   func(name string, args []string) error
}

func (runner *fakeRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	runner.calls = append(runner.calls, call{name: name, args: append([]string(nil), args...)})
	if runner.inspect != nil {
		if err := runner.inspect(name, args); err != nil {
			return "", err
		}
	}
	if len(runner.responses) == 0 {
		return "", nil
	}
	result := runner.responses[0]
	runner.responses = runner.responses[1:]
	return result.output, result.err
}

func dependencies(runner *fakeRunner, tempDir string) (Dependencies, *bytes.Buffer, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer
	return Dependencies{Runner: runner, Stdout: &stdout, Stderr: &stderr, TempDir: tempDir}, &stdout, &stderr
}

func TestValidateGRPCEndpoint(t *testing.T) {
	tests := []struct {
		endpoint string
		wantErr  string
	}{
		{endpoint: "collector.example:4317"},
		{endpoint: "[2001:db8::1]:4317"},
		{endpoint: "https://collector:4317", wantErr: "host:port"},
		{endpoint: "collector", wantErr: "host:port"},
		{endpoint: "user@collector:4317", wantErr: "host:port"},
		{endpoint: "collector/path:4317", wantErr: "host:port"},
		{endpoint: "collector:0", wantErr: "between 1 and 65535"},
		{endpoint: ":4317", wantErr: "host:port"},
	}
	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			err := validateGRPCEndpoint(test.endpoint)
			if test.wantErr == "" && err != nil {
				t.Fatalf("validateGRPCEndpoint() error = %v", err)
			}
			if test.wantErr != "" && (err == nil || !strings.Contains(err.Error(), test.wantErr)) {
				t.Fatalf("validateGRPCEndpoint() error = %v, want %q", err, test.wantErr)
			}
		})
	}
}

func TestProfilerValues(t *testing.T) {
	for _, test := range []struct {
		name     string
		insecure bool
	}{
		{name: "TLS by default"},
		{name: "plaintext opt in", insecure: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			data, err := RenderValues("profiles.example:4317", test.insecure)
			if err != nil {
				t.Fatalf("RenderValues() error = %v", err)
			}
			var values map[string]any
			if err := json.Unmarshal(data, &values); err != nil {
				t.Fatalf("decode values: %v", err)
			}
			if values["mode"] != "daemonset" {
				t.Errorf("mode = %v", values["mode"])
			}
			image := values["image"].(map[string]any)
			if image["repository"] != "otel/opentelemetry-collector-ebpf-profiler" || image["tag"] != CollectorVersion {
				t.Errorf("image = %#v", image)
			}
			command := values["command"].(map[string]any)
			if command["name"] != "otelcol-ebpf-profiler" || !strings.Contains(string(data), "service.profilesSupport") {
				t.Errorf("command = %#v", command)
			}
			config := values["alternateConfig"].(map[string]any)
			if len(config["receivers"].(map[string]any)) != 1 || len(config["extensions"].(map[string]any)) != 1 || len(config["exporters"].(map[string]any)) != 1 {
				t.Errorf("config contains unrelated components: %#v", config)
			}
			if _, exists := config["processors"]; exists {
				t.Errorf("config contains processors unsupported by Collector %s: %#v", CollectorVersion, config["processors"])
			}
			pipeline := config["service"].(map[string]any)["pipelines"].(map[string]any)
			if len(pipeline) != 1 || pipeline["profiles"] == nil {
				t.Errorf("pipelines = %#v", pipeline)
			}
			exporter := config["exporters"].(map[string]any)["otlp/profiles"].(map[string]any)
			if exporter["endpoint"] != "profiles.example:4317" || exporter["tls"].(map[string]any)["insecure"] != test.insecure {
				t.Errorf("exporter = %#v", exporter)
			}
			for name, raw := range values["ports"].(map[string]any) {
				if raw.(map[string]any)["enabled"] != false {
					t.Errorf("port %s enabled: %#v", name, raw)
				}
			}
		})
	}
}

func TestAttachUsesSecureTemporaryValuesAndCleansUp(t *testing.T) {
	tempDir := t.TempDir()
	var valuesPath string
	runner := &fakeRunner{
		responses: []response{{}, {}, {output: "installed\n"}, {output: "profiler-agent"}, {output: "ready\n"}},
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
				return errors.New("temporary values file is not mode 0600")
			}
			data, err := os.ReadFile(valuesPath)
			if err != nil {
				return err
			}
			if !strings.Contains(string(data), `"insecure": false`) || !strings.Contains(string(data), "profiles.example:4317") {
				return errors.New("temporary values file has wrong transport settings")
			}
			return nil
		},
	}
	deps, stdout, stderr := dependencies(runner, tempDir)
	code := Main(context.Background(), []string{"attach", "--endpoint", "profiles.example:4317"}, deps)
	if code != 0 {
		t.Fatalf("Main() = %d\nstderr: %s", code, stderr.String())
	}
	if valuesPath == "" {
		t.Fatal("Helm did not receive a values path")
	}
	if _, err := os.Stat(valuesPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary file still exists: %v", err)
	}
	files, err := filepath.Glob(filepath.Join(tempDir, "*"))
	if err != nil || len(files) != 0 {
		t.Fatalf("temporary directory not empty: %v, %v", files, err)
	}
	if !strings.Contains(stdout.String(), "receiver v0.0.202632") {
		t.Fatalf("stdout = %s", stdout.String())
	}
	install := strings.Join(runner.calls[2].args, " ")
	if !strings.Contains(install, "--version 0.166.0") || !strings.Contains(install, "--namespace profiler-system") {
		t.Fatalf("Helm install args = %s", install)
	}
}

func TestAttachCleansTemporaryFileOnHelmError(t *testing.T) {
	tempDir := t.TempDir()
	runner := &fakeRunner{responses: []response{{}, {}, {err: errors.New("install failed")}}}
	deps, _, stderr := dependencies(runner, tempDir)
	if code := Main(context.Background(), []string{"attach", "--endpoint", "profiles.example:4317"}, deps); code != 1 {
		t.Fatalf("Main() = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "install failed") {
		t.Fatalf("stderr = %s", stderr.String())
	}
	files, _ := filepath.Glob(filepath.Join(tempDir, "*"))
	if len(files) != 0 {
		t.Fatalf("temporary files remain: %v", files)
	}
}

func TestAttachRequiresEndpoint(t *testing.T) {
	deps, _, stderr := dependencies(&fakeRunner{}, t.TempDir())
	if code := Main(context.Background(), []string{"attach"}, deps); code != 1 {
		t.Fatalf("Main() = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "required flag") {
		t.Fatalf("stderr = %s", stderr.String())
	}
}

func TestStatusAndDetachPropagateErrors(t *testing.T) {
	for _, args := range [][]string{{"status"}, {"detach"}} {
		runner := &fakeRunner{responses: []response{{err: errors.New("authentication failed")}}}
		deps, _, stderr := dependencies(runner, t.TempDir())
		if code := Main(context.Background(), args, deps); code != 1 {
			t.Fatalf("Main(%v) = %d, want 1", args, code)
		}
		if !strings.Contains(stderr.String(), "authentication failed") {
			t.Fatalf("stderr = %s", stderr.String())
		}
	}
}

func TestHelp(t *testing.T) {
	deps, stdout, stderr := dependencies(&fakeRunner{}, t.TempDir())
	if code := Main(context.Background(), []string{"--help"}, deps); code != 0 {
		t.Fatalf("Main() = %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "kubectl-profiler") {
		t.Fatalf("help output = %s", stdout.String())
	}
}

func TestVersionDistinguishesReceiverAndUpstream(t *testing.T) {
	deps, stdout, stderr := dependencies(&fakeRunner{}, t.TempDir())
	if code := Main(context.Background(), []string{"version"}, deps); code != 0 {
		t.Fatalf("Main() = %d: %s", code, stderr.String())
	}
	for _, want := range []string{"v0.0.202632", "v0.0.202633", "0.158.0", "0.166.0"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("version output missing %q: %s", want, stdout.String())
		}
	}
}
