package kubectlobi

import (
	"github.com/kakkoyun/zeroins/internal/obiintegration"
	"github.com/kakkoyun/zeroins/internal/output"
	"github.com/spf13/cobra"
)

func newLookupCommand(deps Dependencies) *cobra.Command {
	var outputFlag string
	cmd := &cobra.Command{
		Use:   "lookup <library>",
		Short: "Search the embedded OBI Go-library support matrix",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			format, err := output.Parse(outputFlag)
			if err != nil {
				return err
			}
			obiFormat := obiintegration.FormatTable
			switch format {
			case output.FormatJSON:
				obiFormat = obiintegration.FormatJSON
			case output.FormatMarkdown:
				obiFormat = obiintegration.FormatMarkdown
			}
			matches := obiintegration.Lookup(args[0])
			obiintegration.Render(matches, args[0], obiFormat, deps.Stdout)
			return nil
		},
	}
	cmd.Flags().StringVarP(&outputFlag, "output", "o", "table", "Output format: table, json, or markdown")
	return cmd
}
