// Package doctor implements the zeroins preflight checks.
package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/kakkoyun/zeroins/internal/execx"
)

// Dependencies are the injectable seams for doctor checks.
type Dependencies struct {
	Runner execx.Runner
	Stdout io.Writer
	Stderr io.Writer
}

// Status classifies a check result.
type Status string

const (
	StatusPass Status = "pass"
	StatusWarn Status = "warn"
	StatusFail Status = "fail"
)

// Check is a single preflight check result.
type Check struct {
	Name   string `json:"name"`
	Tool   string `json:"tool"`
	Status Status `json:"status"`
	Detail string `json:"detail"`
}

// Summary aggregates check results.
type Summary struct {
	Pass  int `json:"pass"`
	Warn  int `json:"warn"`
	Fail  int `json:"fail"`
	Total int `json:"total"`
}

// Result is the full doctor output.
type Result struct {
	Checks  []Check `json:"checks"`
	Summary Summary `json:"summary"`
}

// Run executes all preflight checks and writes results.
func Run(ctx context.Context, deps Dependencies, strict bool, format string) error {
	result := runChecks(ctx, deps)

	if strict {
		for i, c := range result.Checks {
			if c.Status == StatusWarn {
				result.Checks[i].Status = StatusFail
				result.Summary.Fail++
				result.Summary.Warn--
			}
		}
	}

	switch format {
	case "json":
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("doctor: encode result: %w", err)
		}
		fmt.Fprintln(deps.Stdout, string(data))
	default:
		renderTable(result, deps.Stdout)
	}

	if result.Summary.Fail > 0 {
		return fmt.Errorf("doctor: %d check(s) failed", result.Summary.Fail)
	}
	return nil
}

func runChecks(ctx context.Context, deps Dependencies) Result {
	var checks []Check

	checks = append(checks, checkTooling(ctx, deps)...)
	checks = append(checks, checkCluster(ctx, deps)...)
	checks = append(checks, checkRBAC(ctx, deps)...)
	checks = append(checks, checkNodes(ctx, deps)...)

	result := Result{Checks: checks}
	for _, c := range checks {
		switch c.Status {
		case StatusPass:
			result.Summary.Pass++
		case StatusWarn:
			result.Summary.Warn++
		case StatusFail:
			result.Summary.Fail++
		}
		result.Summary.Total++
	}
	return result
}

func checkTooling(ctx context.Context, deps Dependencies) []Check {
	var checks []Check

	// kubectl
	kubectlVersion, kubectlErr := deps.Runner.Output(ctx, "kubectl", "version", "--client", "--short")
	if kubectlErr != nil {
		checks = append(checks, Check{Name: "kubectl present", Tool: "all", Status: StatusFail, Detail: "kubectl not found or not executable"})
	} else {
		checks = append(checks, Check{Name: "kubectl present", Tool: "all", Status: StatusPass, Detail: strings.TrimSpace(kubectlVersion)})
	}

	// helm
	helmVersion, helmErr := deps.Runner.Output(ctx, "helm", "version", "--short")
	if helmErr != nil {
		checks = append(checks, Check{Name: "helm present", Tool: "obi,profiler", Status: StatusWarn, Detail: "helm not found; required for attach"})
	} else {
		detail := strings.TrimSpace(helmVersion)
		status := StatusPass
		if !strings.HasPrefix(detail, "v4.") {
			status = StatusWarn
			detail += " (zeroins expects Helm v4.x)"
		}
		checks = append(checks, Check{Name: "helm present", Tool: "obi,profiler", Status: status, Detail: detail})
	}

	// go (informational)
	if version, err := deps.Runner.Output(ctx, "go", "version"); err == nil {
		checks = append(checks, Check{Name: "go version", Tool: "otelc", Status: StatusPass, Detail: strings.TrimSpace(version)})
	} else {
		checks = append(checks, Check{Name: "go version", Tool: "otelc", Status: StatusWarn, Detail: "go not found; required for otelc builds"})
	}

	return checks
}

func checkCluster(ctx context.Context, deps Dependencies) []Check {
	var checks []Check

	contextName, err := deps.Runner.Output(ctx, "kubectl", "config", "current-context")
	if err != nil {
		checks = append(checks, Check{Name: "cluster context", Tool: "all", Status: StatusFail, Detail: "cannot read current Kubernetes context"})
		return checks
	}
	checks = append(checks, Check{Name: "cluster context", Tool: "all", Status: StatusPass, Detail: strings.TrimSpace(contextName)})

	server, err := deps.Runner.Output(ctx, "kubectl", "config", "view", "--minify", "-o", "jsonpath={.clusters[0].cluster.server}")
	if err != nil {
		checks = append(checks, Check{Name: "cluster API server", Tool: "all", Status: StatusWarn, Detail: "cannot read API server from kubeconfig"})
	} else {
		checks = append(checks, Check{Name: "cluster API server", Tool: "all", Status: StatusPass, Detail: strings.TrimSpace(server)})
	}

	// reachability
	if _, err := deps.Runner.Output(ctx, "kubectl", "cluster-info"); err != nil {
		checks = append(checks, Check{Name: "cluster reachable", Tool: "all", Status: StatusFail, Detail: "kubectl cluster-info failed"})
	} else {
		checks = append(checks, Check{Name: "cluster reachable", Tool: "all", Status: StatusPass, Detail: "cluster-info responded"})
	}

	// server version
	if version, err := deps.Runner.Output(ctx, "kubectl", "version", "--short"); err == nil {
		checks = append(checks, Check{Name: "cluster server version", Tool: "all", Status: StatusPass, Detail: strings.TrimSpace(version)})
	} else {
		checks = append(checks, Check{Name: "cluster server version", Tool: "all", Status: StatusWarn, Detail: "could not read server version"})
	}

	return checks
}

func checkRBAC(ctx context.Context, deps Dependencies) []Check {
	var checks []Check

	// Can create DaemonSets
	if out, err := deps.Runner.Output(ctx, "kubectl", "auth", "can-i", "create", "daemonsets"); err != nil {
		checks = append(checks, Check{Name: "RBAC create DaemonSets", Tool: "obi,profiler", Status: StatusWarn, Detail: "cannot check RBAC"})
	} else if strings.TrimSpace(out) == "yes" {
		checks = append(checks, Check{Name: "RBAC create DaemonSets", Tool: "obi,profiler", Status: StatusPass, Detail: "allowed"})
	} else {
		checks = append(checks, Check{Name: "RBAC create DaemonSets", Tool: "obi,profiler", Status: StatusFail, Detail: "not allowed"})
	}

	// Can create ClusterRoles
	if out, err := deps.Runner.Output(ctx, "kubectl", "auth", "can-i", "create", "clusterroles"); err != nil {
		checks = append(checks, Check{Name: "RBAC create ClusterRoles", Tool: "obi", Status: StatusWarn, Detail: "cannot check RBAC"})
	} else if strings.TrimSpace(out) == "yes" {
		checks = append(checks, Check{Name: "RBAC create ClusterRoles", Tool: "obi", Status: StatusPass, Detail: "allowed"})
	} else {
		checks = append(checks, Check{Name: "RBAC create ClusterRoles", Tool: "obi", Status: StatusWarn, Detail: "not allowed; OBI chart may fail"})
	}

	// Can patch Deployments
	if out, err := deps.Runner.Output(ctx, "kubectl", "auth", "can-i", "patch", "deployments"); err != nil {
		checks = append(checks, Check{Name: "RBAC patch Deployments", Tool: "obi", Status: StatusWarn, Detail: "cannot check RBAC"})
	} else if strings.TrimSpace(out) == "yes" {
		checks = append(checks, Check{Name: "RBAC patch Deployments", Tool: "obi", Status: StatusPass, Detail: "allowed"})
	} else {
		checks = append(checks, Check{Name: "RBAC patch Deployments", Tool: "obi", Status: StatusFail, Detail: "not allowed; sidecar mode requires this"})
	}

	return checks
}

type nodeInfo struct {
	Items []struct {
		Status struct {
			NodeInfo struct {
				KernelVersion string `json:"kernelVersion"`
				OSImage       string `json:"operatingSystem"`
				Architecture  string `json:"architecture"`
			} `json:"nodeInfo"`
		} `json:"status"`
	} `json:"items"`
}

func checkNodes(ctx context.Context, deps Dependencies) []Check {
	var checks []Check

	out, err := deps.Runner.Output(ctx, "kubectl", "get", "nodes", "-o", "json")
	if err != nil {
		checks = append(checks, Check{Name: "nodes", Tool: "obi,profiler", Status: StatusFail, Detail: "cannot list nodes"})
		return checks
	}

	var info nodeInfo
	if err := json.Unmarshal([]byte(out), &info); err != nil {
		checks = append(checks, Check{Name: "nodes", Tool: "obi,profiler", Status: StatusFail, Detail: "cannot parse node list"})
		return checks
	}

	if len(info.Items) == 0 {
		checks = append(checks, Check{Name: "nodes", Tool: "obi,profiler", Status: StatusFail, Detail: "no nodes found"})
		return checks
	}

	for _, node := range info.Items {
		ni := node.Status.NodeInfo
		detail := fmt.Sprintf("kernel=%s os=%s arch=%s", ni.KernelVersion, ni.OSImage, ni.Architecture)
		status := StatusPass
		if ni.OSImage != "linux" {
			status = StatusFail
			detail += " (non-Linux: OBI and profiler require Linux)"
		}
		checks = append(checks, Check{Name: "node " + truncateName(ni.KernelVersion, 40), Tool: "obi,profiler", Status: status, Detail: detail})
	}

	return checks
}

func truncateName(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func renderTable(result Result, stdout io.Writer) {
	fmt.Fprintf(stdout, "zeroins doctor\n------------\n\n")
	fmt.Fprintf(stdout, "%-35s  %-12s  %-6s  %s\n", "CHECK", "TOOL", "STATUS", "DETAIL")
	fmt.Fprintln(stdout, strings.Repeat("-", 100))
	for _, c := range result.Checks {
		fmt.Fprintf(stdout, "%-35s  %-12s  %-6s  %s\n", c.Name, c.Tool, c.Status, c.Detail)
	}
	fmt.Fprintf(stdout, "\nSummary: %d pass, %d warn, %d fail (total %d)\n",
		result.Summary.Pass, result.Summary.Warn, result.Summary.Fail, result.Summary.Total)
}
