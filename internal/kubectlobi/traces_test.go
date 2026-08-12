package kubectlobi

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func traceDependencies(serverURL string, client *http.Client) (Dependencies, *bytes.Buffer) {
	var stdout bytes.Buffer
	if client == nil {
		client = http.DefaultClient
	}
	return Dependencies{
		Runner: &fakeRunner{},
		Client: client,
		Stdout: &stdout,
		Stderr: &bytes.Buffer{},
		Getenv: func(key string) string {
			if key == "OTEL_BACKEND" {
				return serverURL
			}
			return ""
		},
	}, &stdout
}

func TestPullTracesSuccessAndNamespaceTag(t *testing.T) {
	var requestURL *url.URL
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestURL = request.URL
		fmt.Fprint(writer, `{"data":[{"spans":[{"operationName":"GET /ready","duration":1500,"startTime":1000000}]}]}`)
	}))
	defer server.Close()
	deps, stdout := traceDependencies(server.URL, nil)
	if err := pullTraces(context.Background(), deps, "checkout service", "prod/team", 7, false); err != nil {
		t.Fatalf("pullTraces() error = %v", err)
	}
	if requestURL.Query().Get("service") != "checkout service" || requestURL.Query().Get("limit") != "7" {
		t.Fatalf("query = %s", requestURL.RawQuery)
	}
	if got := requestURL.Query().Get("tags"); got != `{"k8s.namespace.name":"prod/team"}` {
		t.Fatalf("tags = %q", got)
	}
	if !strings.Contains(stdout.String(), "GET /ready") || !strings.Contains(stdout.String(), "1.5ms") {
		t.Fatalf("stdout = %s", stdout.String())
	}
}

func TestPullTracesEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(writer, `{"data":[]}`)
	}))
	defer server.Close()
	deps, stdout := traceDependencies(server.URL, nil)
	if err := pullTraces(context.Background(), deps, "web", "", 20, false); err != nil {
		t.Fatalf("pullTraces() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "No traces found") {
		t.Fatalf("stdout = %s", stdout.String())
	}
}

func TestPullTracesFailures(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		want       string
	}{
		{name: "malformed JSON", body: `{`, want: "parse response"},
		{name: "HTTP error", statusCode: http.StatusUnauthorized, body: `nope`, want: "401 Unauthorized: nope"},
		{name: "oversized", body: strings.Repeat("x", maxTraceResponseBytes+1), want: "response exceeds"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				if test.statusCode != 0 {
					writer.WriteHeader(test.statusCode)
				}
				fmt.Fprint(writer, test.body)
			}))
			defer server.Close()
			deps, _ := traceDependencies(server.URL, nil)
			err := pullTraces(context.Background(), deps, "web", "", 20, false)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("pullTraces() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestPullTracesTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		fmt.Fprint(writer, `{"data":[]}`)
	}))
	defer server.Close()
	deps, _ := traceDependencies(server.URL, &http.Client{Timeout: 10 * time.Millisecond})
	err := pullTraces(context.Background(), deps, "web", "", 20, false)
	if err == nil || !strings.Contains(err.Error(), "Client.Timeout") {
		t.Fatalf("pullTraces() error = %v, want timeout", err)
	}
}

func TestPullTracesFollowCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(writer, `{"data":[]}`)
	}))
	defer server.Close()
	deps, _ := traceDependencies(server.URL, nil)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	if err := pullTraces(ctx, deps, "web", "", 20, true); err != nil {
		t.Fatalf("pullTraces() error = %v", err)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("cancellation took %s", elapsed)
	}
}

func TestTracesCommandBoundsTail(t *testing.T) {
	for _, tail := range []string{"0", "1001"} {
		deps, _, stderr := testDeps(&fakeRunner{})
		if code := Main(context.Background(), []string{"traces", "web", "--tail", tail}, deps); code != 1 {
			t.Fatalf("Main(--tail=%s) = %d, want 1", tail, code)
		}
		if !strings.Contains(stderr.String(), "between 1 and 1000") {
			t.Fatalf("stderr for --tail=%s: %s", tail, stderr.String())
		}
	}
}

func TestBuildJaegerURLRejectsCredentials(t *testing.T) {
	backend := (&url.URL{Scheme: "https", Host: "jaeger.example", User: url.User("example")}).String()
	_, err := buildJaegerURL(backend, "web", "", 20)
	if err == nil || !strings.Contains(err.Error(), "user information") {
		t.Fatalf("buildJaegerURL() error = %v", err)
	}
}
