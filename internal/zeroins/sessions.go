package zeroins

import (
	"github.com/kakkoyun/zeroins/internal/output"
	"github.com/kakkoyun/zeroins/internal/sessions"
	"github.com/spf13/cobra"
)

func newSessionsCommand(deps Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "List and reap zeroins-managed attach sessions",
	}
	cmd.AddCommand(newSessionsListCommand(deps), newSessionsReapCommand(deps))
	return cmd
}

func newSessionsListCommand(deps Dependencies) *cobra.Command {
	var outputFlag string
	var allNamespaces bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List zeroins-managed attach sessions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			format, err := output.ParseSimple(outputFlag)
			if err != nil {
				return err
			}
			return sessions.List(cmd.Context(), sessions.Dependencies{
				Runner: deps.Runner,
				Stdout: deps.Stdout,
				Stderr: deps.Stderr,
			}, allNamespaces, string(format))
		},
	}
	cmd.Flags().StringVarP(&outputFlag, "output", "o", "table", "Output format: table or json")
	cmd.Flags().BoolVarP(&allNamespaces, "all-namespaces", "A", false, "List across all namespaces")
	return cmd
}

func newSessionsReapCommand(deps Dependencies) *cobra.Command {
	var allNamespaces bool
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "reap",
		Short: "Detach every expired zeroins-managed session",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return sessions.Reap(cmd.Context(), sessions.Dependencies{
				Runner: deps.Runner,
				Stdout: deps.Stdout,
				Stderr: deps.Stderr,
			}, allNamespaces, dryRun)
		},
	}
	cmd.Flags().BoolVarP(&allNamespaces, "all-namespaces", "A", false, "Reap across all namespaces")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print what would be reaped without mutating")
	return cmd
}
