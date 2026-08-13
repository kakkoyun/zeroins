// Package output provides shared output format types and helpers.
package output

import "fmt"

// Format selects the output representation for command results.
type Format string

const (
	// FormatTable emits human-readable aligned tables.
	FormatTable Format = "table"
	// FormatJSON emits structured JSON to stdout.
	FormatJSON Format = "json"
	// FormatMarkdown emits Markdown tables (catalog lookups only).
	FormatMarkdown Format = "markdown"
)

// Parse validates an output format string and returns the typed Format.
func Parse(value string) (Format, error) {
	switch value {
	case "table", "":
		return FormatTable, nil
	case "json":
		return FormatJSON, nil
	case "markdown":
		return FormatMarkdown, nil
	default:
		return "", fmt.Errorf("invalid output format %q: choose table, json, or markdown", value)
	}
}

// ParseSimple validates formats that do not support markdown (status, traces, etc.).
func ParseSimple(value string) (Format, error) {
	switch value {
	case "table", "":
		return FormatTable, nil
	case "json":
		return FormatJSON, nil
	default:
		return "", fmt.Errorf("invalid output format %q: choose table or json", value)
	}
}
