// Package obiintegration provides an offline OBI support-matrix lookup.
package obiintegration

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const (
	OBIVersion = "v0.10.0"
	SourceURL  = "https://github.com/open-telemetry/opentelemetry-ebpf-instrumentation/blob/v0.10.0/SUPPORT_MATRIX.md#go-library-instrumentation"
)

// Format selects the output representation for catalog results.
type Format string

const (
	FormatTable    Format = "table"
	FormatJSON     Format = "json"
	FormatMarkdown Format = "markdown"
)

type entry struct {
	library  string
	baseline string
}

var entries = []entry{
	{library: "net/http", baseline: ">= 1.17"},
	{library: "golang.org/x/net/http2", baseline: ">= 0.12.0"},
	{library: "github.com/gorilla/mux", baseline: ">= v1.5.0"},
	{library: "github.com/gin-gonic/gin", baseline: ">= v1.6.0, != v1.7.5"},
	{library: "google.golang.org/grpc", baseline: ">= 1.40"},
	{library: "net/rpc/jsonrpc", baseline: ">= 1.17"},
	{library: "database/sql", baseline: ">= 1.17"},
	{library: "github.com/go-sql-driver/mysql", baseline: ">= v1.5.0"},
	{library: "github.com/lib/pq", baseline: "all versions"},
	{library: "github.com/redis/go-redis/v9", baseline: ">= v9.0.0"},
	{library: "github.com/segmentio/kafka-go", baseline: ">= v0.4.11"},
	{library: "github.com/IBM/sarama", baseline: ">= 1.37"},
	{library: "go.mongodb.org/mongo-driver", baseline: "v1: >= v1.10.1; v2: >= v2.0.1"},
}

// Match holds a single catalog match.
type Match struct {
	Library  string `json:"library"`
	Baseline string `json:"baseline"`
}

// Lookup searches the catalog and returns matches.
func Lookup(query string) []Match {
	q := strings.ToLower(strings.TrimSpace(query))
	var matches []Match
	for _, candidate := range entries {
		if strings.Contains(strings.ToLower(candidate.library), q) {
			matches = append(matches, Match{Library: candidate.library, Baseline: candidate.baseline})
		}
	}
	return matches
}

// Render writes matches in the selected format. query is the original user input
// for headers and no-match guidance.
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
	fmt.Fprintf(stdout, "OBI %s — integration: %s\n", OBIVersion, query)
	fmt.Fprintf(stdout, "Source: %s\n\n", SourceURL)
	if len(matches) == 0 {
		fmt.Fprintf(stdout, "No OBI integration found for %q.\n\n", query)
		fmt.Fprintf(stdout, "Check the full matrix: %s\n", SourceURL)
		fmt.Fprintln(stdout, "Supported libraries include: net/http, gin, gRPC, gorilla/mux, go-redis, Kafka, database/sql")
		return
	}
	fmt.Fprintf(stdout, "%-45s  %s\n", "LIBRARY", "BASELINE")
	fmt.Fprintln(stdout, strings.Repeat("-", 78))
	for _, m := range matches {
		fmt.Fprintf(stdout, "%-45s  %s\n", m.Library, m.Baseline)
	}
	fmt.Fprintln(stdout, "\nNote: zero code changes covers standard RED metrics and supported library spans.")
	fmt.Fprintln(stdout, "Custom spans, business events, and application-specific attributes still require in-process instrumentation.")
}

func renderMarkdown(matches []Match, query string, stdout io.Writer) {
	fmt.Fprintf(stdout, "# OBI %s — integration: %s\n", OBIVersion, query)
	fmt.Fprintf(stdout, "# Source: %s\n\n", SourceURL)
	if len(matches) == 0 {
		fmt.Fprintf(stdout, "No OBI integration found for %q.\n\n", query)
		fmt.Fprintf(stdout, "Check the full matrix: %s\n", SourceURL)
		fmt.Fprintln(stdout, "Supported libraries include: net/http, gin, gRPC, gorilla/mux, go-redis, Kafka, database/sql")
		return
	}
	fmt.Fprintln(stdout, "| Library | Baseline |")
	fmt.Fprintln(stdout, "| --- | --- |")
	for _, m := range matches {
		fmt.Fprintf(stdout, "| `%s` | `%s` |\n", m.Library, m.Baseline)
	}
	fmt.Fprintln(stdout, "\nNote: zero code changes covers standard RED metrics and supported library spans.")
	fmt.Fprintln(stdout, "Custom spans, business events, and application-specific attributes still require in-process instrumentation.")
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
		Tool:    "obi",
		Version: OBIVersion,
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
		fmt.Fprintln(stderr, "Usage: obi-integration <library-or-import-path>")
		fmt.Fprintln(stderr, "Example: obi-integration net/http")
		return 1
	}
	matches := Lookup(args[0])
	Render(matches, args[0], format, stdout)
	return 0
}
