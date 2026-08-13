package kubectlprofiler

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/kakkoyun/zeroins/internal/execx"
	"github.com/kakkoyun/zeroins/internal/output"
)

// ProfilerPlan describes exactly what a profiler attach or detach would do.
type ProfilerPlan struct {
	Context   string          `json:"context"`
	Namespace string          `json:"namespace"`
	Mode      string          `json:"mode"`
	Endpoint  string          `json:"endpoint,omitempty"`
	Insecure  bool            `json:"insecure,omitempty"`
	Privilege []string        `json:"privilege"`
	Commands  []ProfilerCmd   `json:"commands"`
	Values    json.RawMessage `json:"values,omitempty"`
}

// ProfilerCmd is one external command that would run.
type ProfilerCmd struct {
	Name string   `json:"name"`
	Args []string `json:"args"`
}

func (c ProfilerCmd) String() string {
	return fmt.Sprintf("%s %s", c.Name, joinProfilerArgs(c.Args))
}

func joinProfilerArgs(args []string) string {
	result := ""
	for i, a := range args {
		if i > 0 {
			result += " "
		}
		result += a
	}
	return result
}

func resolveProfilerContext(deps Dependencies) string {
	contextName, err := execx.CurrentContext(deps.Runner)
	if err != nil {
		return ""
	}
	return contextName
}

func profilerPrivilegeImpact() []string {
	return []string{
		"privileged containers on every node",
		"host PID namespace",
		"eBPF capabilities",
		"tracefs mount",
	}
}

func printProfilerPlan(deps Dependencies, namespace, endpoint string, insecure bool, outputFormat string) error {
	values, err := RenderValues(endpoint, insecure)
	if err != nil {
		return fmt.Errorf("dry-run: build Helm values: %w", err)
	}
	plan := ProfilerPlan{
		Context:   resolveProfilerContext(deps),
		Namespace: namespace,
		Mode:      "daemonset",
		Endpoint:  endpoint,
		Insecure:  insecure,
		Privilege: profilerPrivilegeImpact(),
		Commands: []ProfilerCmd{
			{Name: "helm", Args: []string{"repo", "add", "open-telemetry", helmRepoURL}},
			{Name: "helm", Args: []string{"repo", "update", "open-telemetry"}},
			{Name: "helm", Args: []string{"upgrade", "--install", "profiler", chartName, "--version", ChartVersion, "--namespace", namespace, "--create-namespace", "--values", "<temp-file>"}},
			{Name: "kubectl", Args: []string{"get", "daemonset", "-l", "app.kubernetes.io/instance=profiler", "-n", namespace, "-o", "jsonpath={.items[0].metadata.name}"}},
			{Name: "kubectl", Args: []string{"rollout", "status", "daemonset/<daemonset>", "-n", namespace, "--timeout=120s"}},
			{Name: "kubectl", Args: []string{"label", "daemonset", "<daemonset>", "-n", namespace, "zeroins.kakkoyun.dev/managed=true", "--overwrite"}},
			{Name: "kubectl", Args: []string{"annotate", "daemonset", "<daemonset>", "-n", namespace, "--overwrite", "zeroins.kakkoyun.dev/session-id=<auto>", "zeroins.kakkoyun.dev/attached-at=<now>", "zeroins.kakkoyun.dev/endpoint=" + endpoint, "zeroins.kakkoyun.dev/mode=daemonset", "zeroins.kakkoyun.dev/tool=profiler"}},
		},
		Values: json.RawMessage(values),
	}
	return printProfilerPlanOutput(deps, plan, outputFormat)
}

func printProfilerDetachPlan(deps Dependencies, namespace, outputFormat string) error {
	plan := ProfilerPlan{
		Context:   resolveProfilerContext(deps),
		Namespace: namespace,
		Mode:      "daemonset",
		Commands: []ProfilerCmd{
			{Name: "helm", Args: []string{"uninstall", "profiler", "--namespace", namespace, "--ignore-not-found"}},
		},
	}
	return printProfilerPlanOutput(deps, plan, outputFormat)
}

func printProfilerPlanOutput(deps Dependencies, plan ProfilerPlan, outputFormat string) error {
	format, err := output.ParseSimple(outputFormat)
	if err != nil {
		return err
	}
	switch format {
	case output.FormatJSON:
		data, err := json.MarshalIndent(plan, "", "  ")
		if err != nil {
			return fmt.Errorf("render plan: %w", err)
		}
		fmt.Fprintln(deps.Stdout, string(data))
	default:
		renderProfilerPlanTable(plan, deps.Stdout)
	}
	return nil
}

func renderProfilerPlanTable(plan ProfilerPlan, stdout io.Writer) {
	fmt.Fprintf(stdout, "Plan (dry-run, no mutations)\n")
	fmt.Fprintf(stdout, "==========================\n\n")

	fmt.Fprintf(stdout, "1. Resolved target\n")
	fmt.Fprintf(stdout, "   context:   %s\n", plan.Context)
	fmt.Fprintf(stdout, "   namespace: %s\n", plan.Namespace)
	fmt.Fprintf(stdout, "   mode:      %s\n", plan.Mode)
	if plan.Endpoint != "" {
		fmt.Fprintf(stdout, "   endpoint:  %s\n", plan.Endpoint)
		fmt.Fprintf(stdout, "   insecure:  %v\n", plan.Insecure)
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
}
