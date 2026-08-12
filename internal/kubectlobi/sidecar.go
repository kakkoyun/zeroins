package kubectlobi

import (
	"context"
	"encoding/json"
	"fmt"
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
	out, err := deps.Runner.Run(ctx, "kubectl", "get", "deployment", deployment, "-n", namespace, "-o", "json")
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
	document, err := readDeployment(ctx, deps, deployment, namespace)
	if err != nil {
		return fmt.Errorf("attach sidecar: %w", err)
	}
	if document.hasOBIContainer() {
		if !document.managedByZeroins() {
			return fmt.Errorf("attach sidecar: deployment already has an unmanaged container named obi")
		}
		fmt.Fprintf(deps.Stdout, "Deployment %q already has a zeroins-managed OBI sidecar.\n", deployment)
		return nil
	}

	patch := map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"metadata": map[string]any{
					"annotations": map[string]any{
						managedAnnotation:       "true",
						originalShareAnnotation: originalShareValue(document.Spec.Template.Spec.ShareProcessNamespace),
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
	fmt.Fprint(deps.Stdout, out)
	return restartAndWait(ctx, deps, deployment, namespace, "attach sidecar")
}

func detachSidecar(ctx context.Context, deps Dependencies, deployment, namespace string) error {
	document, err := readDeployment(ctx, deps, deployment, namespace)
	if err != nil {
		return fmt.Errorf("detach sidecar: %w", err)
	}
	if !document.hasOBIContainer() {
		fmt.Fprintf(deps.Stdout, "Deployment %q has no OBI sidecar (already detached).\n", deployment)
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
	fmt.Fprint(deps.Stdout, out)
	return restartAndWait(ctx, deps, deployment, namespace, "detach sidecar")
}

func restartAndWait(ctx context.Context, deps Dependencies, deployment, namespace, operation string) error {
	out, err := deps.Runner.Run(ctx, "kubectl", "rollout", "restart", "deployment/"+deployment, "-n", namespace)
	if err != nil {
		return fmt.Errorf("%s: restart deployment: %w", operation, err)
	}
	fmt.Fprint(deps.Stdout, out)
	out, err = deps.Runner.Run(ctx, "kubectl", "rollout", "status", "deployment/"+deployment, "-n", namespace, "--timeout=120s")
	if err != nil {
		return fmt.Errorf("%s: wait for rollout: %w", operation, err)
	}
	fmt.Fprint(deps.Stdout, out)
	return nil
}
