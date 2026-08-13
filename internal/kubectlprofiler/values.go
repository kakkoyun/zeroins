package kubectlprofiler

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newValuesCommand(deps Dependencies) *cobra.Command {
	var endpoint string
	var insecure bool
	cmd := &cobra.Command{
		Use:   "values",
		Short: "Print the exact Helm values that attach would apply",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := validateGRPCEndpoint(endpoint); err != nil {
				return err
			}
			values, err := RenderValues(endpoint, insecure)
			if err != nil {
				return fmt.Errorf("values: build Helm values: %w", err)
			}
			fmt.Fprintln(deps.Stdout, string(values))
			return nil
		},
	}
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "Required OTLP/gRPC profiles destination (host:port)")
	cmd.Flags().BoolVar(&insecure, "insecure", false, "Use plaintext OTLP/gRPC instead of TLS")
	_ = cmd.MarkFlagRequired("endpoint")
	return cmd
}
