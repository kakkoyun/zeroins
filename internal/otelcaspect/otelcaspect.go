// Package otelcaspect provides an offline otelc integration catalog lookup.
package otelcaspect

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const (
	OtelcVersion = "v1.0.1"
	SourceURL    = "https://github.com/open-telemetry/opentelemetry-go-compile-instrumentation/tree/v1.0.1/instrumentation"
)

// Format selects the output representation for catalog results.
type Format string

const (
	FormatTable    Format = "table"
	FormatJSON     Format = "json"
	FormatMarkdown Format = "markdown"
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

// Match holds a single catalog match.
type Match struct {
	Name       string `json:"name"`
	ImportPath string `json:"importPath"`
	Operations string `json:"operations"`
}

// Lookup searches the catalog and returns matches.
func Lookup(query string) []Match {
	q := strings.ToLower(strings.TrimSpace(query))
	var matches []Match
	for _, candidate := range entries {
		haystack := strings.ToLower(candidate.name + " " + candidate.importPath)
		if strings.Contains(haystack, q) {
			matches = append(matches, Match{Name: candidate.name, ImportPath: candidate.importPath, Operations: candidate.operations})
		}
	}
	return matches
}

// Render writes matches in the selected format.
func Render(matches []Match, query string, format Format, stdout io.Writer) {
	switch format {
	case FormatJSON:
		renderJSON(matches, query, stdout)
	case FormatMarkdown:
		renderMarkdown(matches, query, stdout)
	default:
		renderTable(matches, query, stdout)
	}
}

func renderTable(matches []Match, query string, stdout io.Writer) {
	fmt.Fprintf(stdout, "otelc %s — integration: %s\n", OtelcVersion, query)
	fmt.Fprintln(stdout, "Requires: Go 1.25+")
	fmt.Fprintf(stdout, "Source: %s\n\n", SourceURL)
	if len(matches) == 0 {
		fmt.Fprintf(stdout, "No supported otelc %s integration found for %q.\n", OtelcVersion, query)
		fmt.Fprintln(stdout, "Supported libraries are instrumented automatically; unsupported paths require a new instrumentation rule.")
	} else {
		fmt.Fprintf(stdout, "%-30s  %-45s  %s\n", "SUPPORT", "IMPORT PATH", "OPERATIONS")
		fmt.Fprintln(stdout, strings.Repeat("-", 120))
		for _, m := range matches {
			fmt.Fprintf(stdout, "%-30s  %-45s  %s\n", m.Name, m.ImportPath, m.Operations)
		}
	}
	fmt.Fprintln(stdout, "\nBuild with otelc")
	fmt.Fprintf(stdout, "go install go.opentelemetry.io/otelc/tool/cmd/otelc@%s\n", OtelcVersion)
	fmt.Fprintln(stdout, "otelc go build -o ./myapp ./...")
	fmt.Fprintln(stdout, "\nVerify instrumentation")
	fmt.Fprintln(stdout, "otelc go build -v ./... 2>&1 | grep -i inject")
}

func renderMarkdown(matches []Match, query string, stdout io.Writer) {
	fmt.Fprintf(stdout, "# otelc %s — integration: %s\n", OtelcVersion, query)
	fmt.Fprintln(stdout, "# Requires: Go 1.25+")
	fmt.Fprintf(stdout, "# Source: %s\n\n", SourceURL)
	if len(matches) == 0 {
		fmt.Fprintf(stdout, "No supported otelc %s integration found for %q.\n", OtelcVersion, query)
		fmt.Fprintln(stdout, "Supported libraries are instrumented automatically; unsupported paths require a new instrumentation rule.")
	} else {
		fmt.Fprintln(stdout, "| Support | Import path | Instrumented operations |")
		fmt.Fprintln(stdout, "| --- | --- | --- |")
		for _, m := range matches {
			fmt.Fprintf(stdout, "| %s | `%s` | %s |\n", m.Name, m.ImportPath, m.Operations)
		}
	}
	fmt.Fprintln(stdout, "\n## Build with otelc")
	fmt.Fprintf(stdout, "go install go.opentelemetry.io/otelc/tool/cmd/otelc@%s\n", OtelcVersion)
	fmt.Fprintln(stdout, "otelc go build -o ./myapp ./...")
	fmt.Fprintln(stdout, "\n## Verify instrumentation")
	fmt.Fprintln(stdout, "otelc go build -v ./... 2>&1 | grep -i inject")
}

type jsonResult struct {
	Tool    string  `json:"tool"`
	Version string  `json:"version"`
	Query   string  `json:"query"`
	Source  string  `json:"source"`
	Matches []Match `json:"matches"`
}

func renderJSON(matches []Match, query string, stdout io.Writer) {
	result := jsonResult{
		Tool:    "otelc",
		Version: OtelcVersion,
		Query:   query,
		Source:  SourceURL,
		Matches: matches,
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Fprintln(stdout, string(data))
}

// Run executes the catalog lookup and returns a process exit code.
// Deprecated: use Lookup + Render for format control.
func Run(args []string, stdout, stderr io.Writer) int {
	return RunFormat(args, FormatMarkdown, stdout, stderr)
}

// RunFormat executes the catalog lookup with the given output format.
func RunFormat(args []string, format Format, stdout, stderr io.Writer) int {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		fmt.Fprintln(stderr, "Usage: otelc-aspect <library-or-import-path>")
		fmt.Fprintln(stderr, "Example: otelc-aspect net/http")
		return 1
	}
	matches := Lookup(args[0])
	Render(matches, args[0], format, stdout)
	return 0
}
