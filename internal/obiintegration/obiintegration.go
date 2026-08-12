// Package obiintegration provides an offline OBI support-matrix lookup.
package obiintegration

import (
	"fmt"
	"io"
	"strings"
)

const (
	OBIVersion = "v0.10.0"
	SourceURL  = "https://github.com/open-telemetry/opentelemetry-ebpf-instrumentation/blob/v0.10.0/SUPPORT_MATRIX.md#go-library-instrumentation"
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

// Run executes the catalog lookup and returns a process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		fmt.Fprintln(stderr, "Usage: obi-integration <library-or-import-path>")
		fmt.Fprintln(stderr, "Example: obi-integration net/http")
		return 1
	}

	query := strings.ToLower(strings.TrimSpace(args[0]))
	fmt.Fprintf(stdout, "# OBI %s — integration: %s\n", OBIVersion, args[0])
	fmt.Fprintf(stdout, "# Source: %s\n\n", SourceURL)

	var matches []entry
	for _, candidate := range entries {
		if strings.Contains(strings.ToLower(candidate.library), query) {
			matches = append(matches, candidate)
		}
	}
	if len(matches) == 0 {
		fmt.Fprintf(stdout, "No OBI integration found for %q.\n\n", args[0])
		fmt.Fprintf(stdout, "Check the full matrix: %s\n", SourceURL)
		fmt.Fprintln(stdout, "Supported libraries include: net/http, gin, gRPC, gorilla/mux, go-redis, Kafka, database/sql")
		return 0
	}

	fmt.Fprintln(stdout, "| Library | Baseline |")
	fmt.Fprintln(stdout, "| --- | --- |")
	for _, match := range matches {
		fmt.Fprintf(stdout, "| `%s` | `%s` |\n", match.library, match.baseline)
	}
	fmt.Fprintln(stdout, "\nNote: zero code changes covers standard RED metrics and supported library spans.")
	fmt.Fprintln(stdout, "Custom spans, business events, and application-specific attributes still require in-process instrumentation.")
	return 0
}
