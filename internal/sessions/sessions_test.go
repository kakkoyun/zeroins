package sessions

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

type sessCall struct {
	name   string
	args   []string
	output bool
}

type sessResponse struct {
	output string
	err    error
}

type sessRunner struct {
	calls     []sessCall
	responses []sessResponse
}

func (r *sessRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	r.calls = append(r.calls, sessCall{name: name, args: append([]string(nil), args...), output: false})
	return r.respond()
}

func (r *sessRunner) Output(_ context.Context, name string, args ...string) (string, error) {
	r.calls = append(r.calls, sessCall{name: name, args: append([]string(nil), args...), output: true})
	return r.respond()
}

func (r *sessRunner) respond() (string, error) {
	if len(r.responses) == 0 {
		return "", nil
	}
	resp := r.responses[0]
	r.responses = r.responses[1:]
	return resp.output, resp.err
}

func sessDeps(runner *sessRunner) (Dependencies, *bytes.Buffer, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer
	return Dependencies{Runner: runner, Stdout: &stdout, Stderr: &stderr}, &stdout, &stderr
}

func TestSessionsListEmpty(t *testing.T) {
	runner := &sessRunner{responses: []sessResponse{
		{output: `{"items":[]}`},
	}}
	deps, stdout, _ := sessDeps(runner)
	if err := List(context.Background(), deps, true, "table"); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "No zeroins-managed sessions") {
		t.Fatalf("expected empty message: %s", stdout.String())
	}
}

func TestSessionsListActiveAndExpired(t *testing.T) {
	now := time.Now().UTC()
	expired := now.Add(-5 * time.Minute).Format(time.RFC3339)
	active := now.Add(5 * time.Minute).Format(time.RFC3339)
	attached := now.Add(-10 * time.Minute).Format(time.RFC3339)

	managedJSON := `{"items":[
		{"kind":"DaemonSet","metadata":{"name":"obi","namespace":"obi-system","labels":{"` + ManagedLabel + `":"true"},"annotations":{"` + SessionIDAnnotation + `":"obi-abc","` + ToolAnnotation + `":"obi","` + ModeAnnotation + `":"daemonset","` + EndpointAnnotation + `":"https://collector:4318","` + AttachedAtAnnotation + `":"` + attached + `","` + ExpiresAtAnnotation + `":"` + expired + `"}}},
		{"kind":"Deployment","metadata":{"name":"web","namespace":"prod","labels":{"` + ManagedLabel + `":"true"},"annotations":{"` + SessionIDAnnotation + `":"obi-xyz","` + ToolAnnotation + `":"obi","` + ModeAnnotation + `":"sidecar","` + EndpointAnnotation + `":"http://collector:4318","` + AttachedAtAnnotation + `":"` + attached + `","` + ExpiresAtAnnotation + `":"` + active + `"}}}
	]}`

	runner := &sessRunner{responses: []sessResponse{{output: managedJSON}}}
	deps, stdout, _ := sessDeps(runner)
	if err := List(context.Background(), deps, true, "table"); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	output := stdout.String()
	if !strings.Contains(output, "obi-abc") || !strings.Contains(output, "expired") {
		t.Errorf("missing expired session: %s", output)
	}
	if !strings.Contains(output, "obi-xyz") || !strings.Contains(output, "active") {
		t.Errorf("missing active session: %s", output)
	}
}

func TestSessionsListJSON(t *testing.T) {
	attached := time.Now().UTC().Add(-5 * time.Minute).Format(time.RFC3339)
	managedJSON := `{"items":[
		{"kind":"DaemonSet","metadata":{"name":"obi","namespace":"obi-system","labels":{"` + ManagedLabel + `":"true"},"annotations":{"` + SessionIDAnnotation + `":"obi-abc","` + ToolAnnotation + `":"obi","` + ModeAnnotation + `":"daemonset","` + EndpointAnnotation + `":"https://collector:4318","` + AttachedAtAnnotation + `":"` + attached + `"}}}
	]}`
	runner := &sessRunner{responses: []sessResponse{{output: managedJSON}}}
	deps, stdout, _ := sessDeps(runner)
	if err := List(context.Background(), deps, true, "json"); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if !strings.Contains(stdout.String(), `"sessions"`) || !strings.Contains(stdout.String(), `"obi-abc"`) {
		t.Fatalf("json output missing sessions: %s", stdout.String())
	}
}

func TestSessionsReapOnlyExpired(t *testing.T) {
	now := time.Now().UTC()
	expired := now.Add(-5 * time.Minute).Format(time.RFC3339)
	active := now.Add(5 * time.Minute).Format(time.RFC3339)
	attached := now.Add(-10 * time.Minute).Format(time.RFC3339)

	managedJSON := `{"items":[
		{"kind":"DaemonSet","metadata":{"name":"obi","namespace":"obi-system","labels":{"` + ManagedLabel + `":"true"},"annotations":{"` + SessionIDAnnotation + `":"obi-exp","` + ToolAnnotation + `":"obi","` + ModeAnnotation + `":"daemonset","` + EndpointAnnotation + `":"https://c:4318","` + AttachedAtAnnotation + `":"` + attached + `","` + ExpiresAtAnnotation + `":"` + expired + `"}}},
		{"kind":"DaemonSet","metadata":{"name":"profiler","namespace":"profiler-system","labels":{"` + ManagedLabel + `":"true"},"annotations":{"` + SessionIDAnnotation + `":"prof-act","` + ToolAnnotation + `":"profiler","` + ModeAnnotation + `":"daemonset","` + EndpointAnnotation + `":"p:4317","` + AttachedAtAnnotation + `":"` + attached + `","` + ExpiresAtAnnotation + `":"` + active + `"}}}
	]}`

	runner := &sessRunner{responses: []sessResponse{
		{output: managedJSON},   // query sessions
		{output: "uninstalled"}, // helm uninstall obi (expired)
	}}
	deps, _, stderr := sessDeps(runner)
	if err := Reap(context.Background(), deps, true, false); err != nil {
		t.Fatalf("Reap() error = %v", err)
	}
	// Should have made 2 calls: query + 1 detach (only expired)
	if len(runner.calls) != 2 {
		t.Fatalf("calls = %d, want 2: %#v", len(runner.calls), runner.calls)
	}
	if !strings.Contains(stderr.String(), "1 reaped") {
		t.Errorf("expected 1 reaped: %s", stderr.String())
	}
}

func TestSessionsReapDryRunNoMutation(t *testing.T) {
	now := time.Now().UTC()
	expired := now.Add(-5 * time.Minute).Format(time.RFC3339)
	attached := now.Add(-10 * time.Minute).Format(time.RFC3339)

	managedJSON := `{"items":[
		{"kind":"DaemonSet","metadata":{"name":"obi","namespace":"obi-system","labels":{"` + ManagedLabel + `":"true"},"annotations":{"` + SessionIDAnnotation + `":"obi-exp","` + ToolAnnotation + `":"obi","` + ModeAnnotation + `":"daemonset","` + EndpointAnnotation + `":"https://c:4318","` + AttachedAtAnnotation + `":"` + attached + `","` + ExpiresAtAnnotation + `":"` + expired + `"}}}
	]}`

	runner := &sessRunner{responses: []sessResponse{
		{output: managedJSON}, // query sessions only
	}}
	deps, _, stderr := sessDeps(runner)
	if err := Reap(context.Background(), deps, true, true); err != nil {
		t.Fatalf("Reap() error = %v", err)
	}
	// Dry-run: only 1 call (query), no detach
	if len(runner.calls) != 1 {
		t.Fatalf("dry-run calls = %d, want 1: %#v", len(runner.calls), runner.calls)
	}
	if !strings.Contains(stderr.String(), "would reap") {
		t.Errorf("expected 'would reap' in stderr: %s", stderr.String())
	}
}

func TestSessionsReapSkipsNeverExpiring(t *testing.T) {
	attached := time.Now().UTC().Add(-10 * time.Minute).Format(time.RFC3339)
	managedJSON := `{"items":[
		{"kind":"DaemonSet","metadata":{"name":"obi","namespace":"obi-system","labels":{"` + ManagedLabel + `":"true"},"annotations":{"` + SessionIDAnnotation + `":"obi-noexp","` + ToolAnnotation + `":"obi","` + ModeAnnotation + `":"daemonset","` + EndpointAnnotation + `":"https://c:4318","` + AttachedAtAnnotation + `":"` + attached + `"}}}
	]}`

	runner := &sessRunner{responses: []sessResponse{
		{output: managedJSON}, // query sessions only
	}}
	deps, _, stderr := sessDeps(runner)
	if err := Reap(context.Background(), deps, true, false); err != nil {
		t.Fatalf("Reap() error = %v", err)
	}
	// No expires-at: should not detach
	if len(runner.calls) != 1 {
		t.Fatalf("calls = %d, want 1 (no detach for never-expiring): %#v", len(runner.calls), runner.calls)
	}
	if !strings.Contains(stderr.String(), "0 reaped") {
		t.Errorf("expected 0 reaped: %s", stderr.String())
	}
}

func TestSessionsEndpointAnnotationNoCredentials(t *testing.T) {
	attached := time.Now().UTC().Add(-10 * time.Minute).Format(time.RFC3339)
	managedJSON := `{"items":[
		{"kind":"DaemonSet","metadata":{"name":"obi","namespace":"obi-system","labels":{"` + ManagedLabel + `":"true"},"annotations":{"` + SessionIDAnnotation + `":"obi-1","` + ToolAnnotation + `":"obi","` + ModeAnnotation + `":"daemonset","` + EndpointAnnotation + `":"https://collector:4318","` + AttachedAtAnnotation + `":"` + attached + `"}}}
	]}`
	runner := &sessRunner{responses: []sessResponse{{output: managedJSON}}}
	deps, stdout, _ := sessDeps(runner)
	if err := List(context.Background(), deps, true, "json"); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	// The endpoint annotation must not contain credentials
	if strings.Contains(stdout.String(), "user@") || strings.Contains(stdout.String(), "password@") {
		t.Errorf("endpoint annotation contains credentials: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "https://collector:4318") {
		t.Errorf("endpoint annotation missing clean URL: %s", stdout.String())
	}
}
