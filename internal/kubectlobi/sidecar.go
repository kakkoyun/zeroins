package kubectlobi

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/kakkoyun/zeroins/internal/sessions"
)

const (
	managedAnnotation       = "zeroins.kakkoyun.dev/obi-sidecar-managed"
	originalShareAnnotation = "zeroins.kakkoyun.dev/original-share-process-namespace"
	obiImage                = "ghcr.io/open-telemetry/opentelemetry-ebpf-instrumentation/ebpf-instrument:v0.10.0"
)

type deploymentDocument struct {
	Spec struct {
		Template struct {
			Metadata struct {
				Annotations map[string]string `json:"annotations"`
			} `json:"metadata"`
			Spec struct {
				ShareProcessNamespace *bool `json:"shareProcessNamespace"`
				Containers            []struct {
					Name string `json:"name"`
				} `json:"containers"`
			} `json:"spec"`
		} `json:"template"`
	} `json:"spec"`
}

func readDeployment(ctx context.Context, deps Dependencies, deployment, namespace string) (deploymentDocument, error) {
	out, err := deps.Runner.Output(ctx, "kubectl", "get", "deployment", deployment, "-n", namespace, "-o", "json")
	if err != nil {
		return deploymentDocument{}, fmt.Errorf("get deployment: %w", err)
	}
	var document deploymentDocument
	if err := json.Unmarshal([]byte(out), &document); err != nil {
		return deploymentDocument{}, fmt.Errorf("decode deployment: %w", err)
	}
	return document, nil
}

func (document deploymentDocument) hasOBIContainer() bool {
	for _, container := range document.Spec.Template.Spec.Containers {
		if container.Name == "obi" {
			return true
		}
	}
	return false
}

func (document deploymentDocument) managedByZeroins() bool {
	return document.Spec.Template.Metadata.Annotations[managedAnnotation] == "true"
}

func originalShareValue(value *bool) string {
	if value == nil {
		return "unset"
	}
	if *value {
		return "true"
	}
	return "false"
}

func attachSidecar(ctx context.Context, deps Dependencies, deployment, namespace, endpoint string) error {
	return attachSidecarWithMeta(ctx, deps, deployment, namespace, endpoint, 0)
}

func attachSidecarWithMeta(ctx context.Context, deps Dependencies, deployment, namespace, endpoint string, duration time.Duration) error {
	document, err := readDeployment(ctx, deps, deployment, namespace)
	if err != nil {
		return fmt.Errorf("attach sidecar: %w", err)
	}
	if document.hasOBIContainer() {
		if !document.managedByZeroins() {
			return fmt.Errorf("attach sidecar: deployment already has an unmanaged container named obi")
		}
		if duration > 0 {
			return fmt.Errorf("attach sidecar: deployment already has a zeroins-managed OBI sidecar; detach first before starting a bounded re-attach")
		}
		fmt.Fprintf(deps.Stderr, "Deployment %q already has a zeroins-managed OBI sidecar.\n", deployment)
		return nil
	}

	originalShare := originalShareValue(document.Spec.Template.Spec.ShareProcessNamespace)

	patch := map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"metadata": map[string]any{
					"annotations": map[string]any{
						managedAnnotation:       "true",
						originalShareAnnotation: originalShare,
					},
				},
				"spec": map[string]any{
					"shareProcessNamespace": true,
					"containers": []any{map[string]any{
						"name":            "obi",
						"image":           obiImage,
						"imagePullPolicy": "IfNotPresent",
						"securityContext": map[string]any{"privileged": true, "runAsUser": 0},
						"env": []any{
							map[string]any{"name": "OTEL_EXPORTER_OTLP_ENDPOINT", "value": endpoint},
							map[string]any{"name": "OTEL_EBPF_AUTO_TARGET_EXE", "value": "*"},
						},
					}},
				},
			},
		},
	}
	patchJSON, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("attach sidecar: encode patch: %w", err)
	}
	out, err := deps.Runner.Run(ctx, "kubectl", "patch", "deployment", deployment, "-n", namespace, "--type=strategic", "-p", string(patchJSON))
	if err != nil {
		return fmt.Errorf("attach sidecar: patch deployment: %w", err)
	}
	fmt.Fprint(deps.Stderr, out)
	if err := restartAndWait(ctx, deps, deployment, namespace, "attach sidecar"); err != nil {
		return err
	}

	// Compute session metadata after attach completes so attached-at and
	// expires-at reflect the actual start of the bounded window, not the
	// pre-rollout timestamp.
	meta := newSessionMeta(endpoint, "sidecar", duration)
	if err := labelAndAnnotateDeployment(ctx, deps, deployment, namespace, meta); err != nil {
		return fmt.Errorf("attach sidecar: %w", err)
	}
	fmt.Fprintf(deps.Stderr, "Session %s recorded on deployment/%s in %s.\n", meta.ID, deployment, namespace)
	return nil
}

// labelAndAnnotateDeployment applies the managed label and session annotations
// to the Deployment's top-level metadata so sessions list and sessions reap
// can discover sidecar sessions.
func labelAndAnnotateDeployment(ctx context.Context, deps Dependencies, deployment, namespace string, meta sessionMeta) error {
	labelArgs := []string{"label", "deployment", deployment, "-n", namespace, sessions.ManagedLabel + "=" + sessions.ManagedLabelValue, "--overwrite"}
	if _, err := deps.Runner.Run(ctx, "kubectl", labelArgs...); err != nil {
		return fmt.Errorf("label deployment: %w", err)
	}
	annotations := map[string]string{
		sessions.SessionIDAnnotation:  meta.ID,
		sessions.AttachedAtAnnotation: meta.AttachedAt,
		sessions.EndpointAnnotation:   meta.Endpoint,
		sessions.ModeAnnotation:       meta.Mode,
		sessions.ToolAnnotation:       "obi",
	}
	if meta.ExpiresAt != "" {
		annotations[sessions.ExpiresAtAnnotation] = meta.ExpiresAt
	}
	annotateArgs := []string{"annotate", "deployment", deployment, "-n", namespace, "--overwrite"}
	for key, value := range annotations {
		annotateArgs = append(annotateArgs, key+"="+value)
	}
	if _, err := deps.Runner.Run(ctx, "kubectl", annotateArgs...); err != nil {
		return fmt.Errorf("annotate deployment: %w", err)
	}
	return nil
}

func attachSidecarBounded(ctx context.Context, deps Dependencies, deployment, namespace, endpoint string, duration time.Duration) error {
	if err := attachSidecarWithMeta(ctx, deps, deployment, namespace, endpoint, duration); err != nil {
		return err
	}
	return runBoundedAttach(ctx, deps, duration, func(c context.Context) error {
		return detachSidecar(c, deps, deployment, namespace)
	}, fmt.Sprintf("deployment %s in namespace %s", deployment, namespace))
}

func detachSidecar(ctx context.Context, deps Dependencies, deployment, namespace string) error {
	document, err := readDeployment(ctx, deps, deployment, namespace)
	if err != nil {
		return fmt.Errorf("detach sidecar: %w", err)
	}
	if !document.hasOBIContainer() {
		fmt.Fprintf(deps.Stderr, "Deployment %q has no OBI sidecar (already detached).\n", deployment)
		return nil
	}
	if !document.managedByZeroins() {
		return fmt.Errorf("detach sidecar: refusing to remove unmanaged container named obi")
	}

	original := document.Spec.Template.Metadata.Annotations[originalShareAnnotation]
	if original != "unset" && original != "false" && original != "true" {
		return fmt.Errorf("detach sidecar: missing or invalid original shareProcessNamespace annotation")
	}
	var restored any
	switch original {
	case "unset":
		restored = nil
	case "false":
		restored = false
	case "true":
		restored = true
	}
	patch := map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"metadata": map[string]any{
					"annotations": map[string]any{
						managedAnnotation:       nil,
						originalShareAnnotation: nil,
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
		return fmt.Errorf("detach sidecar: encode patch: %w", err)
	}
	out, err := deps.Runner.Run(ctx, "kubectl", "patch", "deployment", deployment, "-n", namespace, "--type=strategic", "-p", string(patchJSON))
	if err != nil {
		return fmt.Errorf("detach sidecar: patch deployment: %w", err)
	}
	fmt.Fprint(deps.Stderr, out)

	// Clear the managed label and session annotations from the Deployment's
	// top-level metadata.
	clearArgs := []string{"annotate", "deployment", deployment, "-n", namespace, "--overwrite",
		sessions.ManagedLabel + "-",
		sessions.SessionIDAnnotation + "-",
		sessions.AttachedAtAnnotation + "-",
		sessions.ExpiresAtAnnotation + "-",
		sessions.EndpointAnnotation + "-",
		sessions.ModeAnnotation + "-",
		sessions.ToolAnnotation + "-",
	}
	if _, err := deps.Runner.Run(ctx, "kubectl", clearArgs...); err != nil {
		// Non-fatal: the sidecar is already removed; stale annotations are
		// harmless and sessions reap ignores resources without the managed label.
		fmt.Fprintf(deps.Stderr, "warning: could not clear session annotations: %v\n", err)
	}

	return restartAndWait(ctx, deps, deployment, namespace, "detach sidecar")
}

func restartAndWait(ctx context.Context, deps Dependencies, deployment, namespace, operation string) error {
	out, err := deps.Runner.Run(ctx, "kubectl", "rollout", "restart", "deployment/"+deployment, "-n", namespace)
	if err != nil {
		return fmt.Errorf("%s: restart deployment: %w", operation, err)
	}
	fmt.Fprint(deps.Stderr, out)
	out, err = deps.Runner.Run(ctx, "kubectl", "rollout", "status", "deployment/"+deployment, "-n", namespace, "--timeout=120s")
	if err != nil {
		return fmt.Errorf("%s: wait for rollout: %w", operation, err)
	}
	fmt.Fprint(deps.Stderr, out)
	return nil
}
