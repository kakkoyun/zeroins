// Package otelcaspect provides an offline otelc integration catalog lookup.
package otelcaspect

import (
	"fmt"
	"io"
	"strings"
)

const (
	OtelcVersion = "v1.0.1"
	SourceURL    = "https://github.com/open-telemetry/opentelemetry-go-compile-instrumentation/tree/v1.0.1/instrumentation"
)

type entry struct {
	name       string
	importPath string
	operations string
}

var entries = []entry{
	{name: "HTTP (standard library)", importPath: "net/http", operations: "client and server requests"},
	{name: "gRPC", importPath: "google.golang.org/grpc", operations: "client and server calls"},
	{name: "SQL databases", importPath: "database/sql", operations: "database calls"},
	{name: "Gin", importPath: "github.com/gin-gonic/gin", operations: "server requests"},
	{name: "Redis", importPath: "github.com/redis/go-redis/v9", operations: "client commands"},
	{name: "MongoDB", importPath: "go.mongodb.org/mongo-driver/mongo", operations: "client commands"},
	{name: "Kafka", importPath: "github.com/segmentio/kafka-go", operations: "produced and consumed messages"},
	{name: "OpenAI", importPath: "github.com/openai/openai-go", operations: "client calls (v1)"},
	{name: "OpenAI", importPath: "github.com/openai/openai-go/v2", operations: "client calls (v2)"},
	{name: "OpenAI", importPath: "github.com/openai/openai-go/v3", operations: "client calls (v3)"},
	{name: "Kubernetes client", importPath: "k8s.io/client-go/tools/cache", operations: "informer cache operations (v0.34.x-v0.35.x)"},
	{name: "log (standard library)", importPath: "log", operations: "log records"},
	{name: "slog (standard library)", importPath: "log/slog", operations: "log records"},
	{name: "Logrus", importPath: "github.com/sirupsen/logrus", operations: "log records"},
}

// Run executes the catalog lookup and returns a process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		fmt.Fprintln(stderr, "Usage: otelc-aspect <library-or-import-path>")
		fmt.Fprintln(stderr, "Example: otelc-aspect net/http")
		return 1
	}

	query := strings.ToLower(strings.TrimSpace(args[0]))
	fmt.Fprintf(stdout, "# otelc %s — integration: %s\n", OtelcVersion, args[0])
	fmt.Fprintln(stdout, "# Requires: Go 1.25+")
	fmt.Fprintf(stdout, "# Source: %s\n\n", SourceURL)

	matches := make([]entry, 0)
	for _, candidate := range entries {
		haystack := strings.ToLower(candidate.name + " " + candidate.importPath)
		if strings.Contains(haystack, query) {
			matches = append(matches, candidate)
		}
	}
	if len(matches) == 0 {
		fmt.Fprintf(stdout, "No supported otelc %s integration found for %q.\n", OtelcVersion, args[0])
		fmt.Fprintln(stdout, "Supported libraries are instrumented automatically; unsupported paths require a new instrumentation rule.")
	} else {
		fmt.Fprintln(stdout, "| Support | Import path | Instrumented operations |")
		fmt.Fprintln(stdout, "| --- | --- | --- |")
		for _, match := range matches {
			fmt.Fprintf(stdout, "| %s | `%s` | %s |\n", match.name, match.importPath, match.operations)
		}
	}

	fmt.Fprintln(stdout, "\n## Build with otelc")
	fmt.Fprintf(stdout, "go install go.opentelemetry.io/otelc/tool/cmd/otelc@%s\n", OtelcVersion)
	fmt.Fprintln(stdout, "otelc go build -o ./myapp ./...")
	fmt.Fprintln(stdout, "\n## Verify instrumentation")
	fmt.Fprintln(stdout, "otelc go build -v ./... 2>&1 | grep -i inject")
	return 0
}
