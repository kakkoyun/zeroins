// Package kubectlobi implements the experimental kubectl-obi command.
package kubectlobi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/kakkoyun/zeroins/internal/execx"
	"github.com/kakkoyun/zeroins/internal/version"
	"github.com/spf13/cobra"
)

const (
	OBIVersion   = "v0.10.0"
	ChartVersion = "0.10.0"
	defaultNS    = "obi-system"
	chartName    = "open-telemetry/opentelemetry-ebpf-instrumentation"
	helmRepoURL  = "https://open-telemetry.github.io/opentelemetry-helm-charts"
)

// Dependencies are the command's injectable process, network, and I/O seams.
type Dependencies struct {
	Runner  execx.Runner
	Client  *http.Client
	Stdout  io.Writer
	Stderr  io.Writer
	Getenv  func(string) string
	TempDir string
}

// DefaultDependencies returns production dependencies.
func DefaultDependencies() Dependencies {
	return Dependencies{
		Runner: execx.OSRunner{},
		Client: &http.Client{Timeout: 10 * time.Second},
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Getenv: os.Getenv,
	}
}

func (deps Dependencies) normalized() Dependencies {
	if deps.Runner == nil {
		deps.Runner = execx.OSRunner{}
	}
	if deps.Client == nil {
		deps.Client = &http.Client{Timeout: 10 * time.Second}
	}
	if deps.Stdout == nil {
		deps.Stdout = io.Discard
	}
	if deps.Stderr == nil {
		deps.Stderr = io.Discard
	}
	if deps.Getenv == nil {
		deps.Getenv = os.Getenv
	}
	return deps
}

// Main executes kubectl-obi and returns its process exit code.
func Main(ctx context.Context, args []string, deps Dependencies) int {
	deps = deps.normalized()
	cmd := NewCommand(deps)
	cmd.SetArgs(args)
	if err := cmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(deps.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

// NewCommand constructs the kubectl-obi command tree.
func NewCommand(deps Dependencies) *cobra.Command {
	deps = deps.normalized()
	root := &cobra.Command{
		Use:           "kubectl-obi",
		Short:         "Manage OBI in a Kubernetes cluster (experimental)",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(deps.Stdout)
	root.SetErr(deps.Stderr)
	root.AddCommand(newAttachCommand(deps), newStatusCommand(deps), newTracesCommand(deps), newDetachCommand(deps), newVersionCommand(deps))
	return root
}

func newAttachCommand(deps Dependencies) *cobra.Command {
	var namespace, mode, endpoint string
	cmd := &cobra.Command{
		Use:   "attach [deployment]",
		Short: "Attach OBI as a DaemonSet or sidecar (experimental)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOTLPEndpoint(endpoint); err != nil {
				return err
			}
			switch mode {
			case "daemonset":
				if len(args) != 0 {
					return fmt.Errorf("daemonset mode does not accept a deployment argument")
				}
				if namespace == "" {
					namespace = defaultNS
				}
				return attachDaemonSet(cmd.Context(), deps, namespace, endpoint)
			case "sidecar":
				if len(args) != 1 {
					return fmt.Errorf("sidecar mode requires exactly one deployment argument")
				}
				if namespace == "" {
					var err error
					namespace, err = execx.CurrentNamespace(cmd.Context(), deps.Runner)
					if err != nil {
						return err
					}
				}
				return attachSidecar(cmd.Context(), deps, args[0], namespace, endpoint)
			default:
				return fmt.Errorf("unknown mode %q; choose daemonset or sidecar", mode)
			}
		},
	}
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace (daemonset default: obi-system; sidecar default: current context)")
	cmd.Flags().StringVar(&mode, "mode", "daemonset", "Deployment mode: daemonset or sidecar")
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "Required OTLP HTTP(S) base endpoint")
	_ = cmd.MarkFlagRequired("endpoint")
	return cmd
}

func validateOTLPEndpoint(endpoint string) error {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("endpoint must be an absolute http or https OTLP URL")
	}
	if parsed.User != nil {
		return fmt.Errorf("endpoint must not contain user information")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("endpoint must not contain a query or fragment")
	}
	path := strings.TrimSuffix(parsed.Path, "/")
	for _, signalPath := range []string{"/v1/logs", "/v1/metrics", "/v1/traces"} {
		if strings.HasSuffix(path, signalPath) {
			return fmt.Errorf("endpoint must be an OTLP base URL, not a signal-specific %s URL", signalPath)
		}
	}
	return nil
}

func signalEndpoint(endpoint, signal string) (string, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("parse endpoint: %w", err)
	}
	parsed.Path = strings.TrimSuffix(parsed.Path, "/") + "/v1/" + signal
	parsed.RawPath = ""
	return parsed.String(), nil
}

func daemonSetValues(endpoint string) ([]byte, error) {
	metricsEndpoint, err := signalEndpoint(endpoint, "metrics")
	if err != nil {
		return nil, err
	}
	tracesEndpoint, err := signalEndpoint(endpoint, "traces")
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"env": map[string]any{
			"OTEL_EXPORTER_OTLP_ENDPOINT":         endpoint,
			"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT": metricsEndpoint,
			"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT":  tracesEndpoint,
		},
	})
}

func attachDaemonSet(ctx context.Context, deps Dependencies, namespace, endpoint string) error {
	values, err := daemonSetValues(endpoint)
	if err != nil {
		return fmt.Errorf("attach daemonset: build Helm values: %w", err)
	}
	file, err := os.CreateTemp(deps.TempDir, "zeroins-obi-values-*.json")
	if err != nil {
		return fmt.Errorf("attach daemonset: create temporary values file: %w", err)
	}
	valuesPath := file.Name()
	defer os.Remove(valuesPath)
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return fmt.Errorf("attach daemonset: secure temporary values file: %w", err)
	}
	if _, err := file.Write(values); err != nil {
		file.Close()
		return fmt.Errorf("attach daemonset: write temporary values file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("attach daemonset: close temporary values file: %w", err)
	}

	fmt.Fprintf(deps.Stdout, "Deploying OBI %s into namespace %q via Helm chart %s...\n", OBIVersion, namespace, ChartVersion)
	if _, err := deps.Runner.Run(ctx, "helm", "repo", "add", "open-telemetry", helmRepoURL); err != nil && !strings.Contains(err.Error(), "already exists") {
		return fmt.Errorf("attach daemonset: add Helm repository: %w", err)
	}
	if _, err := deps.Runner.Run(ctx, "helm", "repo", "update", "open-telemetry"); err != nil {
		return fmt.Errorf("attach daemonset: update Helm repository: %w", err)
	}
	out, err := deps.Runner.Run(ctx, "helm", "upgrade", "--install", "obi", chartName,
		"--version", ChartVersion,
		"--namespace", namespace, "--create-namespace",
		"--values", valuesPath)
	if err != nil {
		return fmt.Errorf("attach daemonset: install Helm chart: %w", err)
	}
	fmt.Fprint(deps.Stdout, out)

	dsName, err := deps.Runner.Run(ctx, "kubectl", "get", "daemonset", "-l", "app.kubernetes.io/instance=obi", "-n", namespace, "-o", "jsonpath={.items[0].metadata.name}")
	if err != nil {
		return fmt.Errorf("attach daemonset: find DaemonSet: %w", err)
	}
	if strings.TrimSpace(dsName) == "" {
		return fmt.Errorf("attach daemonset: Helm release did not create a DaemonSet")
	}
	out, err = deps.Runner.Run(ctx, "kubectl", "rollout", "status", "daemonset/"+strings.TrimSpace(dsName), "-n", namespace, "--timeout=120s")
	if err != nil {
		return fmt.Errorf("attach daemonset: wait for rollout: %w", err)
	}
	fmt.Fprint(deps.Stdout, out)
	return nil
}

func newStatusCommand(deps Dependencies) *cobra.Command {
	var namespace string
	var allNamespaces bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show OBI DaemonSet and pod status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return showStatus(cmd.Context(), deps, namespace, allNamespaces)
		},
	}
	cmd.Flags().StringVarP(&namespace, "namespace", "n", defaultNS, "Namespace to check")
	cmd.Flags().BoolVarP(&allNamespaces, "all-namespaces", "A", false, "Check all namespaces")
	return cmd
}

func showStatus(ctx context.Context, deps Dependencies, namespace string, allNamespaces bool) error {
	namespaceArgs := []string{"-n", namespace}
	if allNamespaces {
		namespaceArgs = []string{"--all-namespaces"}
	}
	fmt.Fprintln(deps.Stdout, "OBI Status\n----------")
	for _, resource := range []string{"daemonset", "pods"} {
		args := []string{"get", resource, "-l", "app.kubernetes.io/instance=obi"}
		args = append(args, namespaceArgs...)
		out, err := deps.Runner.Run(ctx, "kubectl", args...)
		if err != nil {
			return fmt.Errorf("status: get %s: %w", resource, err)
		}
		fmt.Fprintf(deps.Stdout, "\n%s:\n%s", resource, out)
	}
	return nil
}

func newDetachCommand(deps Dependencies) *cobra.Command {
	var namespace, mode string
	cmd := &cobra.Command{
		Use:   "detach [deployment]",
		Short: "Remove a zeroins-managed OBI deployment",
		RunE: func(cmd *cobra.Command, args []string) error {
			switch mode {
			case "daemonset":
				if len(args) != 0 {
					return fmt.Errorf("daemonset mode does not accept a deployment argument")
				}
				if namespace == "" {
					namespace = defaultNS
				}
				return detachDaemonSet(cmd.Context(), deps, namespace)
			case "sidecar":
				if len(args) != 1 {
					return fmt.Errorf("sidecar mode requires exactly one deployment argument")
				}
				if namespace == "" {
					var err error
					namespace, err = execx.CurrentNamespace(cmd.Context(), deps.Runner)
					if err != nil {
						return err
					}
				}
				return detachSidecar(cmd.Context(), deps, args[0], namespace)
			default:
				return fmt.Errorf("unknown mode %q; choose daemonset or sidecar", mode)
			}
		},
	}
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace (daemonset default: obi-system; sidecar default: current context)")
	cmd.Flags().StringVar(&mode, "mode", "daemonset", "Deployment mode: daemonset or sidecar")
	return cmd
}

func detachDaemonSet(ctx context.Context, deps Dependencies, namespace string) error {
	out, err := deps.Runner.Run(ctx, "helm", "uninstall", "obi", "--namespace", namespace, "--ignore-not-found")
	if err != nil {
		return fmt.Errorf("detach daemonset: uninstall Helm release: %w", err)
	}
	if strings.TrimSpace(out) == "" {
		fmt.Fprintln(deps.Stdout, "OBI release not found (already removed).")
	} else {
		fmt.Fprint(deps.Stdout, out)
	}
	return nil
}

func newVersionCommand(deps Dependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print zeroins, OBI, and chart versions",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Fprintf(deps.Stdout, "kubectl-obi: %s\nOBI:         %s\nChart:       %s\n", version.Current(), OBIVersion, ChartVersion)
		},
	}
}
