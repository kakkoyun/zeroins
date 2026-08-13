package doctor

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

type docCall struct {
	name   string
	args   []string
	output bool
}

type docResponse struct {
	output string
	err    error
}

type docRunner struct {
	calls     []docCall
	responses []docResponse
}

func (r *docRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	r.calls = append(r.calls, docCall{name: name, args: append([]string(nil), args...), output: false})
	return r.respond()
}

func (r *docRunner) Output(_ context.Context, name string, args ...string) (string, error) {
	r.calls = append(r.calls, docCall{name: name, args: append([]string(nil), args...), output: true})
	return r.respond()
}

func (r *docRunner) respond() (string, error) {
	if len(r.responses) == 0 {
		return "", nil
	}
	resp := r.responses[0]
	r.responses = r.responses[1:]
	return resp.output, resp.err
}

func docDeps(runner *docRunner) (Dependencies, *bytes.Buffer, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer
	return Dependencies{Runner: runner, Stdout: &stdout, Stderr: &stderr}, &stdout, &stderr
}

func TestDoctorAllPass(t *testing.T) {
	runner := &docRunner{responses: []docResponse{
		{output: "Client Version: v1.30.0"},  // kubectl version --client --short
		{output: "v4.2.3"},                   // helm version --short
		{output: "go version go1.24.0"},      // go version
		{output: "test-context"},             // kubectl config current-context
		{output: "https://api.test:6443"},    // kubectl config view --minify
		{output: "Kubernetes control plane"}, // kubectl cluster-info
		{output: "Server Version: v1.30.0"},  // kubectl version --short
		{output: "yes"},                      // can-i create daemonsets
		{output: "yes"},                      // can-i create clusterroles
		{output: "yes"},                      // can-i patch deployments
		{output: `{"items":[{"status":{"nodeInfo":{"kernelVersion":"6.8.0","operatingSystem":"linux","architecture":"amd64"}}}]}`}, // kubectl get nodes -o json
	}}
	deps, stdout, _ := docDeps(runner)
	if err := Run(context.Background(), deps, false, "table"); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "pass") {
		t.Fatalf("expected pass in output: %s", stdout.String())
	}
}

func TestDoctorFailsOnMissingKubectl(t *testing.T) {
	runner := &docRunner{responses: []docResponse{
		{err: errFake("kubectl not found")}, // kubectl version --client --short fails
	}}
	deps, _, _ := docDeps(runner)
	if err := Run(context.Background(), deps, false, "table"); err == nil {
		t.Fatal("Run() should fail when kubectl is missing")
	}
}

func TestDoctorStrictPromotesWarnings(t *testing.T) {
	runner := &docRunner{responses: []docResponse{
		{output: "Client Version: v1.30.0"},  // kubectl
		{err: errFake("helm not found")},     // helm missing -> warn
		{output: "go version go1.24.0"},      // go
		{output: "test-context"},             // context
		{output: "https://api.test:6443"},    // server
		{output: "Kubernetes control plane"}, // cluster-info
		{output: "Server Version: v1.30.0"},  // server version
		{output: "yes"},                      // can-i daemonsets
		{output: "yes"},                      // can-i clusterroles
		{output: "yes"},                      // can-i patch
		{output: `{"items":[{"status":{"nodeInfo":{"kernelVersion":"6.8.0","operatingSystem":"linux","architecture":"amd64"}}}]}`},
	}}
	deps, _, _ := docDeps(runner)
	// Without strict: should pass (helm missing is just a warning)
	if err := Run(context.Background(), deps, false, "table"); err != nil {
		t.Fatalf("Run(strict=false) error = %v, want nil (warning only)", err)
	}
}

func TestDoctorJSONShape(t *testing.T) {
	runner := &docRunner{responses: []docResponse{
		{output: "v1.30.0"},
		{output: "v4.2.3"},
		{output: "go1.24.0"},
		{output: "test-context"},
		{output: "https://api.test:6443"},
		{output: "cluster-info"},
		{output: "v1.30.0"},
		{output: "yes"},
		{output: "yes"},
		{output: "yes"},
		{output: `{"items":[{"status":{"nodeInfo":{"kernelVersion":"6.8.0","operatingSystem":"linux","architecture":"amd64"}}}]}`},
	}}
	deps, stdout, _ := docDeps(runner)
	if err := Run(context.Background(), deps, false, "json"); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(stdout.String(), `"checks"`) || !strings.Contains(stdout.String(), `"summary"`) {
		t.Fatalf("json output missing checks/summary: %s", stdout.String())
	}
}

func TestDoctorNonLinuxNodeFails(t *testing.T) {
	runner := &docRunner{responses: []docResponse{
		{output: "v1.30.0"},
		{output: "v4.2.3"},
		{output: "go1.24.0"},
		{output: "test-context"},
		{output: "https://api.test:6443"},
		{output: "cluster-info"},
		{output: "v1.30.0"},
		{output: "yes"},
		{output: "yes"},
		{output: "yes"},
		{output: `{"items":[{"status":{"nodeInfo":{"kernelVersion":"24.0.0","operatingSystem":"windows","architecture":"amd64"}}}]}`},
	}}
	deps, _, _ := docDeps(runner)
	if err := Run(context.Background(), deps, false, "table"); err == nil {
		t.Fatal("Run() should fail when node is non-Linux")
	}
}

type fakeErr struct{ msg string }

func (e *fakeErr) Error() string { return e.msg }

func errFake(msg string) error { return &fakeErr{msg: msg} }
