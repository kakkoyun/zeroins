package zeroins

import (
	"github.com/kakkoyun/zeroins/internal/doctor"
	"github.com/kakkoyun/zeroins/internal/output"
	"github.com/spf13/cobra"
)

func newDoctorCommand(deps Dependencies) *cobra.Command {
	var outputFlag string
	var strict bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Preflight: check tooling, cluster, RBAC, and nodes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			format, err := output.ParseSimple(outputFlag)
			if err != nil {
				return err
			}
			return doctor.Run(cmd.Context(), doctor.Dependencies{
				Runner: deps.Runner,
				Stdout: deps.Stdout,
				Stderr: deps.Stderr,
			}, strict, string(format))
		},
	}
	cmd.Flags().StringVarP(&outputFlag, "output", "o", "table", "Output format: table or json")
	cmd.Flags().BoolVar(&strict, "strict", false, "Promote warnings to failures")
	return cmd
}
