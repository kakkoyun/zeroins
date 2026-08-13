// Package kubectlprofiler implements the experimental kubectl-profiler command.
package kubectlprofiler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kakkoyun/zeroins/internal/execx"
	"github.com/kakkoyun/zeroins/internal/output"
	"github.com/kakkoyun/zeroins/internal/version"
	"github.com/spf13/cobra"
)

const (
	ChartVersion            = "0.166.0"
	CollectorVersion        = "0.158.0"
	ProfilerReceiverVersion = "v0.0.202632"
	UpstreamProfilerVersion = "v0.0.202633"
	defaultNamespace        = "profiler-system"
	chartName               = "open-telemetry/opentelemetry-collector"
	helmRepoURL             = "https://open-telemetry.github.io/opentelemetry-helm-charts"
)

// Dependencies are injectable process and I/O seams.
type Dependencies struct {
	Runner  execx.Runner
	Stdout  io.Writer
	Stderr  io.Writer
	TempDir string
}

// DefaultDependencies returns production dependencies.
func DefaultDependencies() Dependencies {
	return Dependencies{Runner: execx.OSRunner{}, Stdout: os.Stdout, Stderr: os.Stderr}
}

func (deps Dependencies) normalized() Dependencies {
	if deps.Runner == nil {
		deps.Runner = execx.OSRunner{}
	}
	if deps.Stdout == nil {
		deps.Stdout = io.Discard
	}
	if deps.Stderr == nil {
		deps.Stderr = io.Discard
	}
	return deps
}

// Main executes kubectl-profiler and returns its process exit code.
func Main(ctx context.Context, args []string, deps Dependencies) int {
	deps = deps.normalized()
	cmd := NewCommand(deps, "kubectl-profiler")
	cmd.SetArgs(args)
	if err := cmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(deps.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

// NewCommand constructs the command tree. The use string sets the root command
// name so the same subtree mounts correctly as "kubectl-profiler" or as
// "profiler" under the unified zeroins root.
func NewCommand(deps Dependencies, use string) *cobra.Command {
	deps = deps.normalized()
	root := &cobra.Command{
		Use:           use,
		Short:         "Manage the OpenTelemetry eBPF Profiler (experimental)",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(deps.Stdout)
	root.SetErr(deps.Stderr)
	root.AddCommand(newAttachCommand(deps), newStatusCommand(deps), newValuesCommand(deps), newDetachCommand(deps), newVersionCommand(deps, use))
	return root
}

func newAttachCommand(deps Dependencies) *cobra.Command {
	var namespace, endpoint, outputFormat string
	var insecure bool
	var dryRun bool
	var duration time.Duration
	cmd := &cobra.Command{
		Use:   "attach",
		Short: "Deploy the dedicated eBPF profiling Collector DaemonSet",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateGRPCEndpoint(endpoint); err != nil {
				return err
			}
			if dryRun {
				return printProfilerPlan(deps, namespace, endpoint, insecure, outputFormat)
			}
			if duration > 0 {
				return attachBounded(cmd.Context(), deps, namespace, endpoint, insecure, duration)
			}
			return attach(cmd.Context(), deps, namespace, endpoint, insecure)
		},
	}
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "Required OTLP/gRPC profiles destination (host:port)")
	cmd.Flags().BoolVar(&insecure, "insecure", false, "Use plaintext OTLP/gRPC instead of TLS")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", defaultNamespace, "Namespace for profiler components")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print the plan without mutating")
	cmd.Flags().DurationVar(&duration, "duration", 0, "Attach for this duration, then detach automatically")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "table", "Output format for --dry-run: table or json")
	_ = cmd.MarkFlagRequired("endpoint")
	return cmd
}

func validateGRPCEndpoint(endpoint string) error {
	host, portText, err := net.SplitHostPort(endpoint)
	if err != nil || host == "" || strings.ContainsAny(host, "@/\\?#") || strings.ContainsAny(host, " \t\r\n") {
		return fmt.Errorf("endpoint must be an OTLP/gRPC host:port (IPv6 must use brackets)")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("endpoint port must be between 1 and 65535")
	}
	return nil
}

func attach(ctx context.Context, deps Dependencies, namespace, endpoint string, insecure bool) error {
	values, err := RenderValues(endpoint, insecure)
	if err != nil {
		return fmt.Errorf("attach: build Helm values: %w", err)
	}
	file, err := os.CreateTemp(deps.TempDir, "zeroins-profiler-values-*.json")
	if err != nil {
		return fmt.Errorf("attach: create temporary values file: %w", err)
	}
	path := file.Name()
	defer os.Remove(path)
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return fmt.Errorf("attach: secure temporary values file: %w", err)
	}
	if _, err := file.Write(values); err != nil {
		file.Close()
		return fmt.Errorf("attach: write temporary values file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("attach: close temporary values file: %w", err)
	}

	fmt.Fprintf(deps.Stderr, "Deploying profiling Collector %s (receiver %s) with chart %s into namespace %q...\n", CollectorVersion, ProfilerReceiverVersion, ChartVersion, namespace)
	if _, err := deps.Runner.Run(ctx, "helm", "repo", "add", "open-telemetry", helmRepoURL); err != nil && !strings.Contains(err.Error(), "already exists") {
		return fmt.Errorf("attach: add Helm repository: %w", err)
	}
	if _, err := deps.Runner.Run(ctx, "helm", "repo", "update", "open-telemetry"); err != nil {
		return fmt.Errorf("attach: update Helm repository: %w", err)
	}
	out, err := deps.Runner.Run(ctx, "helm", "upgrade", "--install", "profiler", chartName,
		"--version", ChartVersion, "--namespace", namespace, "--create-namespace", "--values", path)
	if err != nil {
		return fmt.Errorf("attach: install Helm chart: %w", err)
	}
	fmt.Fprint(deps.Stderr, out)

	dsName, err := deps.Runner.Output(ctx, "kubectl", "get", "daemonset", "-l", "app.kubernetes.io/instance=profiler", "-n", namespace, "-o", "jsonpath={.items[0].metadata.name}")
	if err != nil {
		return fmt.Errorf("attach: find DaemonSet: %w", err)
	}
	if strings.TrimSpace(dsName) == "" {
		return fmt.Errorf("attach: Helm release did not create a DaemonSet")
	}
	out, err = deps.Runner.Run(ctx, "kubectl", "rollout", "status", "daemonset/"+strings.TrimSpace(dsName), "-n", namespace, "--timeout=120s")
	if err != nil {
		return fmt.Errorf("attach: wait for rollout: %w", err)
	}
	fmt.Fprint(deps.Stderr, out)

	meta := newProfilerSessionMeta(endpoint, 0)
	if err := labelAndAnnotateProfilerDaemonSet(ctx, deps, strings.TrimSpace(dsName), namespace, meta); err != nil {
		return fmt.Errorf("attach: %w", err)
	}
	fmt.Fprintf(deps.Stderr, "Session %s recorded on daemonset/%s in %s.\n", meta.ID, strings.TrimSpace(dsName), namespace)
	return nil
}

func attachBounded(ctx context.Context, deps Dependencies, namespace, endpoint string, insecure bool, duration time.Duration) error {
	if err := attachWithDuration(ctx, deps, namespace, endpoint, insecure, duration); err != nil {
		return err
	}
	return runBoundedProfilerAttach(ctx, deps, duration, func(c context.Context) error {
		return detach(c, deps, namespace)
	}, fmt.Sprintf("Helm release profiler in namespace %s", namespace))
}

func attachWithDuration(ctx context.Context, deps Dependencies, namespace, endpoint string, insecure bool, duration time.Duration) error {
	values, err := RenderValues(endpoint, insecure)
	if err != nil {
		return fmt.Errorf("attach: build Helm values: %w", err)
	}
	file, err := os.CreateTemp(deps.TempDir, "zeroins-profiler-values-*.json")
	if err != nil {
		return fmt.Errorf("attach: create temporary values file: %w", err)
	}
	path := file.Name()
	defer os.Remove(path)
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return fmt.Errorf("attach: secure temporary values file: %w", err)
	}
	if _, err := file.Write(values); err != nil {
		file.Close()
		return fmt.Errorf("attach: write temporary values file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("attach: close temporary values file: %w", err)
	}

	fmt.Fprintf(deps.Stderr, "Deploying profiling Collector %s (receiver %s) with chart %s into namespace %q...\n", CollectorVersion, ProfilerReceiverVersion, ChartVersion, namespace)
	if _, err := deps.Runner.Run(ctx, "helm", "repo", "add", "open-telemetry", helmRepoURL); err != nil && !strings.Contains(err.Error(), "already exists") {
		return fmt.Errorf("attach: add Helm repository: %w", err)
	}
	if _, err := deps.Runner.Run(ctx, "helm", "repo", "update", "open-telemetry"); err != nil {
		return fmt.Errorf("attach: update Helm repository: %w", err)
	}
	out, err := deps.Runner.Run(ctx, "helm", "upgrade", "--install", "profiler", chartName,
		"--version", ChartVersion, "--namespace", namespace, "--create-namespace", "--values", path)
	if err != nil {
		return fmt.Errorf("attach: install Helm chart: %w", err)
	}
	fmt.Fprint(deps.Stderr, out)

	dsName, err := deps.Runner.Output(ctx, "kubectl", "get", "daemonset", "-l", "app.kubernetes.io/instance=profiler", "-n", namespace, "-o", "jsonpath={.items[0].metadata.name}")
	if err != nil {
		return fmt.Errorf("attach: find DaemonSet: %w", err)
	}
	if strings.TrimSpace(dsName) == "" {
		return fmt.Errorf("attach: Helm release did not create a DaemonSet")
	}
	out, err = deps.Runner.Run(ctx, "kubectl", "rollout", "status", "daemonset/"+strings.TrimSpace(dsName), "-n", namespace, "--timeout=120s")
	if err != nil {
		return fmt.Errorf("attach: wait for rollout: %w", err)
	}
	fmt.Fprint(deps.Stderr, out)

	meta := newProfilerSessionMeta(endpoint, duration)
	if err := labelAndAnnotateProfilerDaemonSet(ctx, deps, strings.TrimSpace(dsName), namespace, meta); err != nil {
		return fmt.Errorf("attach: %w", err)
	}
	fmt.Fprintf(deps.Stderr, "Session %s recorded on daemonset/%s in %s.\n", meta.ID, strings.TrimSpace(dsName), namespace)
	return nil
}

// RenderValues returns the JSON Helm values used by the profiler deployment.
// It is exported only within this module's internal package boundary so contract
// tests can render the exact configuration used by the command.
func RenderValues(endpoint string, insecure bool) ([]byte, error) {
	disabledPort := func() map[string]any { return map[string]any{"enabled": false} }
	values := map[string]any{
		"mode":    "daemonset",
		"presets": map[string]any{"profiling": map[string]any{"enabled": true}},
		"image": map[string]any{
			"repository": "otel/opentelemetry-collector-ebpf-profiler",
			"tag":        CollectorVersion,
		},
		"command": map[string]any{
			"name":      "otelcol-ebpf-profiler",
			"extraArgs": []string{"--feature-gates=+service.profilesSupport"},
		},
		"alternateConfig": map[string]any{
			"receivers": map[string]any{"profiling": map[string]any{}},
			"exporters": map[string]any{
				"otlp/profiles": map[string]any{
					"endpoint": endpoint,
					"tls":      map[string]any{"insecure": insecure},
				},
			},
			"extensions": map[string]any{
				"health_check": map[string]any{"endpoint": "${env:MY_POD_IP}:13133"},
			},
			"service": map[string]any{
				"extensions": []string{"health_check"},
				"pipelines": map[string]any{
					"profiles": map[string]any{
						"receivers": []string{"profiling"},
						"exporters": []string{"otlp/profiles"},
					},
				},
			},
		},
		"ports": map[string]any{
			"otlp":           disabledPort(),
			"otlp-http":      disabledPort(),
			"jaeger-compact": disabledPort(),
			"jaeger-thrift":  disabledPort(),
			"jaeger-grpc":    disabledPort(),
			"zipkin":         disabledPort(),
			"metrics":        disabledPort(),
		},
		"service": map[string]any{"enabled": false},
	}
	return json.MarshalIndent(values, "", "  ")
}

func newStatusCommand(deps Dependencies) *cobra.Command {
	var namespace string
	var allNamespaces bool
	var outputFormat string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show profiler DaemonSet and pod status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			format, err := output.ParseSimple(outputFormat)
			if err != nil {
				return err
			}
			namespaceArgs := []string{"-n", namespace}
			if allNamespaces {
				namespaceArgs = []string{"--all-namespaces"}
			}
			if format == output.FormatJSON {
				args := []string{"get", "daemonset,pods", "-l", "app.kubernetes.io/instance=profiler"}
				args = append(args, namespaceArgs...)
				args = append(args, "-o", "json")
				out, err := deps.Runner.Output(cmd.Context(), "kubectl", args...)
				if err != nil {
					return fmt.Errorf("status: get resources: %w", err)
				}
				fmt.Fprintln(deps.Stdout, out)
				return nil
			}
			for _, resource := range []string{"daemonset", "pods"} {
				args := []string{"get", resource, "-l", "app.kubernetes.io/instance=profiler"}
				args = append(args, namespaceArgs...)
				out, err := deps.Runner.Run(cmd.Context(), "kubectl", args...)
				if err != nil {
					return fmt.Errorf("status: get %s: %w", resource, err)
				}
				fmt.Fprintf(deps.Stdout, "%s:\n%s", resource, out)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&namespace, "namespace", "n", defaultNamespace, "Namespace to check")
	cmd.Flags().BoolVarP(&allNamespaces, "all-namespaces", "A", false, "Check all namespaces")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "table", "Output format: table or json")
	return cmd
}

func newDetachCommand(deps Dependencies) *cobra.Command {
	var namespace, outputFormat string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "detach",
		Short: "Remove the profiler Helm release",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if dryRun {
				return printProfilerDetachPlan(deps, namespace, outputFormat)
			}
			return detach(cmd.Context(), deps, namespace)
		},
	}
	cmd.Flags().StringVarP(&namespace, "namespace", "n", defaultNamespace, "Namespace of profiler components")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print the plan without mutating")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "table", "Output format for --dry-run: table or json")
	return cmd
}

func detach(ctx context.Context, deps Dependencies, namespace string) error {
	out, err := deps.Runner.Run(ctx, "helm", "uninstall", "profiler", "--namespace", namespace, "--ignore-not-found")
	if err != nil {
		return fmt.Errorf("detach: uninstall Helm release: %w", err)
	}
	if strings.TrimSpace(out) == "" {
		fmt.Fprintln(deps.Stderr, "Profiler release not found (already removed).")
	} else {
		fmt.Fprint(deps.Stderr, out)
	}
	return nil
}

func newVersionCommand(deps Dependencies, use string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print zeroins and pinned profiler component versions",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Fprintf(deps.Stdout, "%s: %s\nCollector:          %s\nProfiler receiver:  %s\nProfiler upstream:  %s\nChart:              %s\n", use, version.Current(), CollectorVersion, ProfilerReceiverVersion, UpstreamProfilerVersion, ChartVersion)
		},
	}
}
