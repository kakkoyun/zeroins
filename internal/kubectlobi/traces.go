package kubectlobi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	maxTraceResponseBytes = 4 << 20
	maxTraceLimit         = 1000
)

type jaegerResponse struct {
	Data []struct {
		Spans []struct {
			OperationName string `json:"operationName"`
			Duration      int64  `json:"duration"`
			StartTime     int64  `json:"startTime"`
		} `json:"spans"`
	} `json:"data"`
}

func newTracesCommand(deps Dependencies) *cobra.Command {
	var namespace string
	var tail int
	var follow bool
	cmd := &cobra.Command{
		Use:   "traces <deployment>",
		Short: "Query recent Jaeger traces for a deployment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if tail < 1 || tail > maxTraceLimit {
				return fmt.Errorf("tail must be between 1 and %d", maxTraceLimit)
			}
			return pullTraces(cmd.Context(), deps, args[0], namespace, tail, follow)
		},
	}
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Filter by k8s.namespace.name")
	cmd.Flags().IntVar(&tail, "tail", 20, "Maximum number of traces to request")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Poll for new traces until cancelled")
	return cmd
}

func pullTraces(ctx context.Context, deps Dependencies, deployment, namespace string, tail int, follow bool) error {
	backend := deps.Getenv("OTEL_BACKEND")
	if backend == "" {
		backend = "http://localhost:16686"
	}
	queryURL, err := buildJaegerURL(backend, deployment, namespace, tail)
	if err != nil {
		return err
	}

	fetch := func() error {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, queryURL, nil)
		if err != nil {
			return fmt.Errorf("pull traces: build request: %w", err)
		}
		response, err := deps.Client.Do(request)
		if err != nil {
			return fmt.Errorf("pull traces: query Jaeger: %w", err)
		}
		defer response.Body.Close()

		body, err := io.ReadAll(io.LimitReader(response.Body, maxTraceResponseBytes+1))
		if err != nil {
			return fmt.Errorf("pull traces: read response: %w", err)
		}
		if len(body) > maxTraceResponseBytes {
			return fmt.Errorf("pull traces: response exceeds %d bytes", maxTraceResponseBytes)
		}
		if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
			message := strings.TrimSpace(string(body))
			if message == "" {
				message = http.StatusText(response.StatusCode)
			}
			return fmt.Errorf("pull traces: Jaeger returned %s: %s", response.Status, message)
		}

		var result jaegerResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return fmt.Errorf("pull traces: parse response: %w", err)
		}
		if len(result.Data) == 0 {
			fmt.Fprintln(deps.Stdout, "No traces found.")
			return nil
		}
		fmt.Fprintf(deps.Stdout, "%-50s  %-12s  %s\n", "OPERATION", "DURATION", "START TIME")
		fmt.Fprintln(deps.Stdout, strings.Repeat("-", 82))
		for _, trace := range result.Data {
			for _, span := range trace.Spans {
				duration := time.Duration(span.Duration) * time.Microsecond
				start := time.UnixMicro(span.StartTime).Format(time.RFC3339)
				fmt.Fprintf(deps.Stdout, "%-50s  %-12s  %s\n", span.OperationName, duration, start)
			}
		}
		return nil
	}

	for {
		if err := fetch(); err != nil {
			if follow && errors.Is(ctx.Err(), context.Canceled) {
				return nil
			}
			return err
		}
		if !follow {
			return nil
		}
		timer := time.NewTimer(2 * time.Second)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return nil
		case <-timer.C:
		}
	}
}

func buildJaegerURL(backend, deployment, namespace string, tail int) (string, error) {
	parsed, err := url.Parse(backend)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("OTEL_BACKEND must be an absolute http or https URL")
	}
	if parsed.User != nil {
		return "", fmt.Errorf("OTEL_BACKEND must not contain user information")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/api/traces"
	query := parsed.Query()
	query.Set("service", deployment)
	query.Set("limit", strconv.Itoa(tail))
	if namespace != "" {
		tags, err := json.Marshal(map[string]string{"k8s.namespace.name": namespace})
		if err != nil {
			return "", fmt.Errorf("encode namespace tag: %w", err)
		}
		query.Set("tags", string(tags))
	}
	parsed.RawQuery = query.Encode()
	parsed.Fragment = ""
	return parsed.String(), nil
}
