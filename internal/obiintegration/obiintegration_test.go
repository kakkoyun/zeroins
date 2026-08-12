package obiintegration

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
		{name: "exact", args: []string{"net/http"}, contains: []string{"OBI v0.10.0", "| `net/http` | `>= 1.17` |", SourceURL}},
		{name: "partial case insensitive", args: []string{"GITHUB.COM"}, contains: []string{"gorilla/mux", "gin-gonic/gin", "go-sql-driver/mysql", "segmentio/kafka-go", "IBM/sarama"}},
		{name: "unknown", args: []string{"example.invalid/package"}, contains: []string{"No OBI integration found", "Supported libraries include"}},
		{name: "missing", wantCode: 1, contains: []string{"Usage: obi-integration"}},
		{name: "too many", args: []string{"net/http", "grpc"}, wantCode: 1, contains: []string{"Usage: obi-integration"}},
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
