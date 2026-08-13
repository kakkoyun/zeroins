package kubectlobi

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newValuesCommand(deps Dependencies) *cobra.Command {
	var mode, endpoint string
	cmd := &cobra.Command{
		Use:   "values",
		Short: "Print the exact Helm values or sidecar patch that attach would apply",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := validateOTLPEndpoint(endpoint); err != nil {
				return err
			}
			switch mode {
			case "daemonset":
				values, err := daemonSetValues(endpoint)
				if err != nil {
					return fmt.Errorf("values: build Helm values: %w", err)
				}
				fmt.Fprintln(deps.Stdout, string(values))
				return nil
			case "sidecar":
				patch := sidecarPatchTemplate(endpoint)
				data, err := json.MarshalIndent(patch, "", "  ")
				if err != nil {
					return fmt.Errorf("values: encode sidecar patch: %w", err)
				}
				fmt.Fprintln(deps.Stdout, string(data))
				return nil
			default:
				return fmt.Errorf("unknown mode %q; choose daemonset or sidecar", mode)
			}
		},
	}
	cmd.Flags().StringVar(&mode, "mode", "daemonset", "Deployment mode: daemonset or sidecar")
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "Required OTLP HTTP(S) base endpoint")
	_ = cmd.MarkFlagRequired("endpoint")
	return cmd
}

// sidecarPatchTemplate returns the strategic merge patch that attach would
// apply for sidecar mode. The original-share-process-namespace annotation
// depends on the live deployment state and is set to "unset" here as a
// placeholder; attach fills it from the cluster.
func sidecarPatchTemplate(endpoint string) map[string]any {
	return map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"metadata": map[string]any{
					"annotations": map[string]any{
						managedAnnotation:       "true",
						originalShareAnnotation: "unset",
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
}
