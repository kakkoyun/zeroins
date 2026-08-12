package otelcaspect

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantCode int
		contains []string
	}{
		{name: "exact", args: []string{"net/http"}, contains: []string{"otelc v1.0.1", "HTTP (standard library)", "client and server requests", SourceURL}},
		{name: "partial case insensitive", args: []string{"GITHUB.COM"}, contains: []string{"gin-gonic/gin", "redis/go-redis", "segmentio/kafka-go", "openai/openai-go", "openai/openai-go/v2", "openai/openai-go/v3", "sirupsen/logrus"}},
		{name: "absent from pinned tag", args: []string{"anthropic"}, contains: []string{"No supported otelc v1.0.1 integration"}},
		{name: "unknown", args: []string{"example.invalid/package"}, contains: []string{"No supported otelc v1.0.1 integration", "otelc go build"}},
		{name: "missing", wantCode: 1, contains: []string{"Usage: otelc-aspect"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(test.args, &stdout, &stderr)
			if code != test.wantCode {
				t.Fatalf("Run() = %d, want %d\nstdout: %s\nstderr: %s", code, test.wantCode, stdout.String(), stderr.String())
			}
			output := stdout.String() + stderr.String()
			for _, want := range test.contains {
				if !strings.Contains(output, want) {
					t.Errorf("output does not contain %q:\n%s", want, output)
				}
			}
		})
	}
}
