// Package sessions implements cluster-recorded attach session tracking.
package sessions

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/kakkoyun/zeroins/internal/execx"
)

// Annotation and label keys applied to managed resources.
const (
	ManagedLabel         = "zeroins.kakkoyun.dev/managed"
	ManagedLabelValue    = "true"
	SessionIDAnnotation  = "zeroins.kakkoyun.dev/session-id"
	AttachedAtAnnotation = "zeroins.kakkoyun.dev/attached-at"
	ExpiresAtAnnotation  = "zeroins.kakkoyun.dev/expires-at"
	EndpointAnnotation   = "zeroins.kakkoyun.dev/endpoint"
	ModeAnnotation       = "zeroins.kakkoyun.dev/mode"
	ToolAnnotation       = "zeroins.kakkoyun.dev/tool"
)

// Dependencies are the injectable seams for session operations.
type Dependencies struct {
	Runner execx.Runner
	Stdout io.Writer
	Stderr io.Writer
}

// Session represents a single zeroins-managed attach session.
type Session struct {
	ID         string `json:"session-id"`
	Tool       string `json:"tool"`
	Mode       string `json:"mode"`
	Namespace  string `json:"namespace"`
	Target     string `json:"target"`
	Endpoint   string `json:"endpoint"`
	AttachedAt string `json:"attached-at"`
	ExpiresAt  string `json:"expires-at,omitempty"`
	State      string `json:"state"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
}

// ListResult holds the list output.
type ListResult struct {
	Sessions []Session `json:"sessions"`
}

// nowFunc returns the current time. Overridable for tests.
var nowFunc = time.Now

// List queries the cluster for zeroins-managed sessions.
func List(ctx context.Context, deps Dependencies, allNamespaces bool, format string) error {
	sessions, err := querySessions(ctx, deps, allNamespaces)
	if err != nil {
		return err
	}

	switch format {
	case "json":
		data, err := json.MarshalIndent(ListResult{Sessions: sessions}, "", "  ")
		if err != nil {
			return fmt.Errorf("sessions list: encode result: %w", err)
		}
		fmt.Fprintln(deps.Stdout, string(data))
	default:
		renderSessionTable(sessions, deps.Stdout)
	}
	return nil
}

// Reap detaches every expired managed session.
func Reap(ctx context.Context, deps Dependencies, allNamespaces, dryRun bool) error {
	sessions, err := querySessions(ctx, deps, allNamespaces)
	if err != nil {
		return err
	}

	now := nowFunc()
	var reaped, skipped int
	for _, s := range sessions {
		if s.ExpiresAt == "" {
			skipped++
			continue
		}
		expiresAt, err := time.Parse(time.RFC3339, s.ExpiresAt)
		if err != nil {
			fmt.Fprintf(deps.Stderr, "session %s: invalid expires-at %q, skipping\n", s.ID, s.ExpiresAt)
			skipped++
			continue
		}
		if expiresAt.After(now) {
			skipped++
			continue
		}
		if dryRun {
			fmt.Fprintf(deps.Stderr, "would reap expired session %s (tool=%s mode=%s namespace=%s target=%s)\n", s.ID, s.Tool, s.Mode, s.Namespace, s.Target)
			reaped++
			continue
		}
		if err := detachSession(ctx, deps, s); err != nil {
			fmt.Fprintf(deps.Stderr, "session %s: detach failed: %v\n", s.ID, err)
			continue
		}
		fmt.Fprintf(deps.Stderr, "reaped expired session %s\n", s.ID)
		reaped++
	}

	fmt.Fprintf(deps.Stderr, "sessions reap: %d reaped, %d skipped\n", reaped, skipped)
	return nil
}

type resourceList struct {
	Items []struct {
		Kind       string `json:"kind"`
		APIVersion string `json:"apiVersion"`
		Metadata   struct {
			Name        string            `json:"name"`
			Namespace   string            `json:"namespace"`
			Labels      map[string]string `json:"labels"`
			Annotations map[string]string `json:"annotations"`
		} `json:"metadata"`
	} `json:"items"`
}

func querySessions(ctx context.Context, deps Dependencies, allNamespaces bool) ([]Session, error) {
	args := []string{"get", "daemonsets,deployments"}
	if allNamespaces {
		args = append(args, "-A")
	}
	args = append(args, "-l", ManagedLabel+"="+ManagedLabelValue, "-o", "json")

	out, err := deps.Runner.Output(ctx, "kubectl", args...)
	if err != nil {
		return nil, fmt.Errorf("sessions: list managed resources: %w", err)
	}

	var list resourceList
	if err := json.Unmarshal([]byte(out), &list); err != nil {
		return nil, fmt.Errorf("sessions: parse resource list: %w", err)
	}

	now := nowFunc()
	var sessions []Session
	for _, item := range list.Items {
		ann := item.Metadata.Annotations
		if ann[ManagedLabel] != ManagedLabelValue && item.Metadata.Labels[ManagedLabel] != ManagedLabelValue {
			continue
		}

		s := Session{
			ID:         ann[SessionIDAnnotation],
			Tool:       ann[ToolAnnotation],
			Mode:       ann[ModeAnnotation],
			Namespace:  item.Metadata.Namespace,
			Target:     item.Metadata.Name,
			Endpoint:   ann[EndpointAnnotation],
			AttachedAt: ann[AttachedAtAnnotation],
			ExpiresAt:  ann[ExpiresAtAnnotation],
			Kind:       item.Kind,
			Name:       item.Metadata.Name,
		}

		s.State = "active"
		if s.ExpiresAt != "" {
			if expiresAt, err := time.Parse(time.RFC3339, s.ExpiresAt); err == nil && expiresAt.Before(now) {
				s.State = "expired"
			}
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

func detachSession(ctx context.Context, deps Dependencies, s Session) error {
	switch s.Tool {
	case "obi":
		return detachOBISession(ctx, deps, s)
	case "profiler":
		return detachProfilerSession(ctx, deps, s)
	default:
		return fmt.Errorf("unknown tool %q for session %s", s.Tool, s.ID)
	}
}

func detachOBISession(ctx context.Context, deps Dependencies, s Session) error {
	switch s.Mode {
	case "daemonset":
		_, err := deps.Runner.Run(ctx, "helm", "uninstall", "obi", "--namespace", s.Namespace, "--ignore-not-found")
		if err != nil {
			return fmt.Errorf("uninstall OBI Helm release: %w", err)
		}
		return nil
	case "sidecar":
		// Read the deployment to get the original shareProcessNamespace annotation
		out, err := deps.Runner.Output(ctx, "kubectl", "get", "deployment", s.Target, "-n", s.Namespace, "-o", "json")
		if err != nil {
			return fmt.Errorf("get deployment: %w", err)
		}
		var doc struct {
			Spec struct {
				Template struct {
					Metadata struct {
						Annotations map[string]string `json:"annotations"`
					} `json:"metadata"`
				} `json:"template"`
			} `json:"spec"`
		}
		if err := json.Unmarshal([]byte(out), &doc); err != nil {
			return fmt.Errorf("decode deployment: %w", err)
		}
		original := doc.Spec.Template.Metadata.Annotations["zeroins.kakkoyun.dev/original-share-process-namespace"]
		var restored any
		switch original {
		case "unset":
			restored = nil
		case "false":
			restored = false
		case "true":
			restored = true
		default:
			return fmt.Errorf("missing or invalid original shareProcessNamespace annotation")
		}
		patch := map[string]any{
			"spec": map[string]any{
				"template": map[string]any{
					"metadata": map[string]any{
						"annotations": map[string]any{
							"zeroins.kakkoyun.dev/obi-sidecar-managed":              nil,
							"zeroins.kakkoyun.dev/original-share-process-namespace": nil,
							SessionIDAnnotation:  nil,
							AttachedAtAnnotation: nil,
							ExpiresAtAnnotation:  nil,
							EndpointAnnotation:   nil,
							ModeAnnotation:       nil,
							ToolAnnotation:       nil,
							ManagedLabel:         nil,
						},
					},
					"spec": map[string]any{
						"shareProcessNamespace": restored,
						"containers":            []any{map[string]any{"name": "obi", "$patch": "delete"}},
					},
				},
			},
		}
		patchJSON, err := json.Marshal(patch)
		if err != nil {
			return fmt.Errorf("encode patch: %w", err)
		}
		_, err = deps.Runner.Run(ctx, "kubectl", "patch", "deployment", s.Target, "-n", s.Namespace, "--type=strategic", "-p", string(patchJSON))
		if err != nil {
			return fmt.Errorf("patch deployment: %w", err)
		}
		_, err = deps.Runner.Run(ctx, "kubectl", "rollout", "restart", "deployment/"+s.Target, "-n", s.Namespace)
		if err != nil {
			return fmt.Errorf("restart deployment: %w", err)
		}
		_, err = deps.Runner.Run(ctx, "kubectl", "rollout", "status", "deployment/"+s.Target, "-n", s.Namespace, "--timeout=120s")
		if err != nil {
			return fmt.Errorf("wait for rollout: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unknown OBI mode %q", s.Mode)
	}
}

func detachProfilerSession(ctx context.Context, deps Dependencies, s Session) error {
	_, err := deps.Runner.Run(ctx, "helm", "uninstall", "profiler", "--namespace", s.Namespace, "--ignore-not-found")
	if err != nil {
		return fmt.Errorf("uninstall profiler Helm release: %w", err)
	}
	return nil
}

func renderSessionTable(sessions []Session, stdout io.Writer) {
	if len(sessions) == 0 {
		fmt.Fprintln(stdout, "No zeroins-managed sessions found.")
		return
	}
	fmt.Fprintf(stdout, "%-12s  %-8s  %-10s  %-15s  %-20s  %-30s  %-20s  %-20s  %s\n",
		"SESSION", "TOOL", "MODE", "NAMESPACE", "TARGET", "ENDPOINT", "ATTACHED", "EXPIRES", "STATE")
	fmt.Fprintln(stdout, strings.Repeat("-", 160))
	for _, s := range sessions {
		fmt.Fprintf(stdout, "%-12s  %-8s  %-10s  %-15s  %-20s  %-30s  %-20s  %-20s  %s\n",
			s.ID, s.Tool, s.Mode, s.Namespace, s.Target, s.Endpoint, s.AttachedAt, s.ExpiresAt, s.State)
	}
}
