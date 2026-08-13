package kubectlobi

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/kakkoyun/zeroins/internal/execx"
	"github.com/kakkoyun/zeroins/internal/output"
)

// Plan describes exactly what an attach or detach would do, without mutating.
type Plan struct {
	Context    string          `json:"context"`
	Namespace  string          `json:"namespace"`
	Mode       string          `json:"mode"`
	Deployment string          `json:"deployment,omitempty"`
	Endpoint   string          `json:"endpoint,omitempty"`
	Privilege  []string        `json:"privilege"`
	Commands   []CommandArgv   `json:"commands"`
	Values     json.RawMessage `json:"values,omitempty"`
	Patch      json.RawMessage `json:"patch,omitempty"`
}

// CommandArgv is one external command that would run.
type CommandArgv struct {
	Name string   `json:"name"`
	Args []string `json:"args"`
}

func (c CommandArgv) String() string {
	return fmt.Sprintf("%s %s", c.Name, joinArgs(c.Args))
}

func joinArgs(args []string) string {
	quoted := make([]string, len(args))
	for i, a := range args {
		if needsQuote(a) {
			quoted[i] = fmt.Sprintf("%q", a)
		} else {
			quoted[i] = a
		}
	}
	return join(quoted, " ")
}

func needsQuote(s string) bool {
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '"' || r == '\'' || r == '\\' {
			return true
		}
	}
	return false
}

func join(parts []string, sep string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += sep
		}
		result += p
	}
	return result
}

// resolveContext reads the current kubectl context name (read-only).
func resolveContext(deps Dependencies) string {
	contextName, err := execx.CurrentContext(deps.Runner)
	if err != nil {
		return ""
	}
	return contextName
}

// renderPlanTable writes the plan in human-readable form.
func renderPlanTable(plan Plan, stdout io.Writer) {
	fmt.Fprintf(stdout, "Plan (dry-run, no mutations)\n")
	fmt.Fprintf(stdout, "==========================\n\n")

	fmt.Fprintf(stdout, "1. Resolved target\n")
	fmt.Fprintf(stdout, "   context:    %s\n", plan.Context)
	fmt.Fprintf(stdout, "   namespace:  %s\n", plan.Namespace)
	fmt.Fprintf(stdout, "   mode:       %s\n", plan.Mode)
	if plan.Deployment != "" {
		fmt.Fprintf(stdout, "   deployment: %s\n", plan.Deployment)
	}
	if plan.Endpoint != "" {
		fmt.Fprintf(stdout, "   endpoint:   %s\n", plan.Endpoint)
	}
	fmt.Fprintln(stdout)

	fmt.Fprintf(stdout, "2. Privilege impact\n")
	for _, p := range plan.Privilege {
		fmt.Fprintf(stdout, "   - %s\n", p)
	}
	fmt.Fprintln(stdout)

	fmt.Fprintf(stdout, "3. Commands that would run\n")
	for _, c := range plan.Commands {
		fmt.Fprintf(stdout, "   %s\n", c.String())
	}
	fmt.Fprintln(stdout)

	if len(plan.Values) > 0 {
		fmt.Fprintf(stdout, "4. Rendered Helm values\n")
		fmt.Fprintln(stdout, "```json")
		fmt.Fprintln(stdout, string(plan.Values))
		fmt.Fprintln(stdout, "```")
	}
	if len(plan.Patch) > 0 {
		fmt.Fprintf(stdout, "4. Sidecar JSON patch\n")
		fmt.Fprintln(stdout, "```json")
		fmt.Fprintln(stdout, string(plan.Patch))
		fmt.Fprintln(stdout, "```")
	}
}

// renderPlanJSON writes the plan as a structured JSON object.
func renderPlanJSON(plan Plan, stdout io.Writer) error {
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Errorf("render plan: %w", err)
	}
	fmt.Fprintln(stdout, string(data))
	return nil
}

func printPlan(deps Dependencies, plan Plan, outputFormat string) error {
	format, err := output.ParseSimple(outputFormat)
	if err != nil {
		return err
	}
	switch format {
	case output.FormatJSON:
		return renderPlanJSON(plan, deps.Stdout)
	default:
		renderPlanTable(plan, deps.Stdout)
		return nil
	}
}

func daemonSetPrivilegeImpact() []string {
	return []string{
		"privileged containers on every node",
		"host PID namespace",
		"eBPF capabilities",
		"tracefs mount",
	}
}

func sidecarPrivilegeImpact() []string {
	return []string{
		"privileged container in the deployment pod",
		"host PID namespace",
		"shareProcessNamespace set to true",
		"deployment rollout (pod restart)",
	}
}

func printDaemonSetPlan(deps Dependencies, namespace, endpoint, outputFormat string) error {
	values, err := daemonSetValues(endpoint)
	if err != nil {
		return fmt.Errorf("dry-run: build Helm values: %w", err)
	}
	plan := Plan{
		Context:   resolveContext(deps),
		Namespace: namespace,
		Mode:      "daemonset",
		Endpoint:  endpoint,
		Privilege: daemonSetPrivilegeImpact(),
		Commands: []CommandArgv{
			{Name: "helm", Args: []string{"repo", "add", "open-telemetry", helmRepoURL}},
			{Name: "helm", Args: []string{"repo", "update", "open-telemetry"}},
			{Name: "helm", Args: []string{"upgrade", "--install", "obi", chartName, "--version", ChartVersion, "--namespace", namespace, "--create-namespace", "--values", "<temp-file>"}},
			{Name: "kubectl", Args: []string{"get", "daemonset", "-l", "app.kubernetes.io/instance=obi", "-n", namespace, "-o", "jsonpath={.items[0].metadata.name}"}},
			{Name: "kubectl", Args: []string{"rollout", "status", "daemonset/<daemonset>", "-n", namespace, "--timeout=120s"}},
			{Name: "kubectl", Args: []string{"label", "daemonset", "<daemonset>", "-n", namespace, "zeroins.kakkoyun.dev/managed=true", "--overwrite"}},
			{Name: "kubectl", Args: []string{"annotate", "daemonset", "<daemonset>", "-n", namespace, "--overwrite", "zeroins.kakkoyun.dev/session-id=<auto>", "zeroins.kakkoyun.dev/attached-at=<now>", "zeroins.kakkoyun.dev/endpoint=" + endpoint, "zeroins.kakkoyun.dev/mode=daemonset", "zeroins.kakkoyun.dev/tool=obi"}},
		},
		Values: json.RawMessage(values),
	}
	return printPlan(deps, plan, outputFormat)
}

func printSidecarPlan(deps Dependencies, deployment, namespace, endpoint, outputFormat string) error {
	patch := sidecarPatchTemplate(endpoint)
	patchJSON, err := json.MarshalIndent(patch, "", "  ")
	if err != nil {
		return fmt.Errorf("dry-run: encode sidecar patch: %w", err)
	}
	plan := Plan{
		Context:    resolveContext(deps),
		Namespace:  namespace,
		Mode:       "sidecar",
		Deployment: deployment,
		Endpoint:   endpoint,
		Privilege:  sidecarPrivilegeImpact(),
		Commands: []CommandArgv{
			{Name: "kubectl", Args: []string{"get", "deployment", deployment, "-n", namespace, "-o", "json"}},
			{Name: "kubectl", Args: []string{"patch", "deployment", deployment, "-n", namespace, "--type=strategic", "-p", "<patch-json>"}},
			{Name: "kubectl", Args: []string{"rollout", "restart", "deployment/" + deployment, "-n", namespace}},
			{Name: "kubectl", Args: []string{"rollout", "status", "deployment/" + deployment, "-n", namespace, "--timeout=120s"}},
		},
		Patch: json.RawMessage(patchJSON),
	}
	return printPlan(deps, plan, outputFormat)
}

func printDaemonSetDetachPlan(deps Dependencies, namespace, outputFormat string) error {
	plan := Plan{
		Context:   resolveContext(deps),
		Namespace: namespace,
		Mode:      "daemonset",
		Commands: []CommandArgv{
			{Name: "helm", Args: []string{"uninstall", "obi", "--namespace", namespace, "--ignore-not-found"}},
		},
	}
	return printPlan(deps, plan, outputFormat)
}

func printSidecarDetachPlan(deps Dependencies, deployment, namespace, outputFormat string) error {
	plan := Plan{
		Context:    resolveContext(deps),
		Namespace:  namespace,
		Mode:       "sidecar",
		Deployment: deployment,
		Commands: []CommandArgv{
			{Name: "kubectl", Args: []string{"get", "deployment", deployment, "-n", namespace, "-o", "json"}},
			{Name: "kubectl", Args: []string{"patch", "deployment", deployment, "-n", namespace, "--type=strategic", "-p", "<patch-json>"}},
			{Name: "kubectl", Args: []string{"rollout", "restart", "deployment/" + deployment, "-n", namespace}},
			{Name: "kubectl", Args: []string{"rollout", "status", "deployment/" + deployment, "-n", namespace, "--timeout=120s"}},
		},
	}
	return printPlan(deps, plan, outputFormat)
}
