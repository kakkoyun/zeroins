// Package zeroins implements the unified zeroins command tree.
package zeroins

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/kakkoyun/zeroins/internal/execx"
	"github.com/kakkoyun/zeroins/internal/kubectlobi"
	"github.com/kakkoyun/zeroins/internal/kubectlprofiler"
	"github.com/kakkoyun/zeroins/internal/otelcaspect"
	"github.com/kakkoyun/zeroins/internal/output"
	"github.com/kakkoyun/zeroins/internal/version"
	"github.com/spf13/cobra"
)

// Dependencies are the injectable process, network, and I/O seams for the
// unified zeroins command.
type Dependencies struct {
	Runner  execx.Runner
	Client  *http.Client
	Stdout  io.Writer
	Stderr  io.Writer
	Getenv  func(string) string
	TempDir string
}

// DefaultDependencies returns production dependencies.
func DefaultDependencies() Dependencies {
	return Dependencies{
		Runner: execx.OSRunner{},
		Client: &http.Client{Timeout: 10 * time.Second},
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Getenv: os.Getenv,
	}
}

func (deps Dependencies) normalized() Dependencies {
	if deps.Runner == nil {
		deps.Runner = execx.OSRunner{}
	}
	if deps.Client == nil {
		deps.Client = &http.Client{Timeout: 10 * time.Second}
	}
	if deps.Stdout == nil {
		deps.Stdout = io.Discard
	}
	if deps.Stderr == nil {
		deps.Stderr = io.Discard
	}
	if deps.Getenv == nil {
		deps.Getenv = os.Getenv
	}
	return deps
}

func (deps Dependencies) obiDeps() kubectlobi.Dependencies {
	return kubectlobi.Dependencies{
		Runner:  deps.Runner,
		Client:  deps.Client,
		Stdout:  deps.Stdout,
		Stderr:  deps.Stderr,
		Getenv:  deps.Getenv,
		TempDir: deps.TempDir,
	}
}

func (deps Dependencies) profilerDeps() kubectlprofiler.Dependencies {
	return kubectlprofiler.Dependencies{
		Runner:  deps.Runner,
		Stdout:  deps.Stdout,
		Stderr:  deps.Stderr,
		TempDir: deps.TempDir,
	}
}

// Main executes zeroins and returns its process exit code.
func Main(ctx context.Context, args []string, deps Dependencies) int {
	deps = deps.normalized()
	cmd := NewCommand(deps)
	cmd.SetArgs(args)
	if err := cmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(deps.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

// NewCommand constructs the unified zeroins command tree.
func NewCommand(deps Dependencies) *cobra.Command {
	deps = deps.normalized()
	root := &cobra.Command{
		Use:           "zeroins",
		Short:         "Observe Go services without changing their source",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(deps.Stdout)
	root.SetErr(deps.Stderr)

	obiRoot := kubectlobi.NewCommand(deps.obiDeps(), "obi")

	profilerRoot := kubectlprofiler.NewCommand(deps.profilerDeps(), "profiler")

	root.AddCommand(
		obiRoot,
		profilerRoot,
		newOtelcCommand(deps),
		newDoctorCommand(deps),
		newSessionsCommand(deps),
		newVersionCommand(deps),
	)
	return root
}

func newVersionCommand(deps Dependencies) *cobra.Command {
	var outputFlag string
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print zeroins and pinned component versions",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			format, err := output.ParseSimple(outputFlag)
			if err != nil {
				return err
			}
			switch format {
			case output.FormatJSON:
				fmt.Fprintf(deps.Stdout, `{"tool":"zeroins","version":%q,"obi":%q,"obiChart":%q,"otelc":%q,"profilerCollector":%q,"profilerChart":%q}`+"\n",
					version.Current(), kubectlobi.OBIVersion, kubectlobi.ChartVersion, otelcaspect.OtelcVersion, kubectlprofiler.CollectorVersion, kubectlprofiler.ChartVersion)
			default:
				fmt.Fprintf(deps.Stdout, "zeroins: %s\n", version.Current())
				fmt.Fprintf(deps.Stdout, "OBI:    %s (chart %s)\n", kubectlobi.OBIVersion, kubectlobi.ChartVersion)
				fmt.Fprintf(deps.Stdout, "otelc:  %s\n", otelcaspect.OtelcVersion)
				fmt.Fprintf(deps.Stdout, "profiler: collector %s (chart %s)\n", kubectlprofiler.CollectorVersion, kubectlprofiler.ChartVersion)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&outputFlag, "output", "o", "table", "Output format: table or json")
	return cmd
}

func newOtelcCommand(deps Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "otelc",
		Short: "Search the embedded otelc supported-library catalog",
	}
	cmd.AddCommand(newOtelcLookupCommand(deps))
	return cmd
}

func newOtelcLookupCommand(deps Dependencies) *cobra.Command {
	var outputFlag string
	cmd := &cobra.Command{
		Use:   "lookup <library>",
		Short: "Search the embedded otelc supported-library catalog",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			format, err := output.Parse(outputFlag)
			if err != nil {
				return err
			}
			otelcFormat := otelcaspect.FormatTable
			switch format {
			case output.FormatJSON:
				otelcFormat = otelcaspect.FormatJSON
			case output.FormatMarkdown:
				otelcFormat = otelcaspect.FormatMarkdown
			}
			matches := otelcaspect.Lookup(args[0])
			otelcaspect.Render(matches, args[0], otelcFormat, deps.Stdout)
			return nil
		},
	}
	cmd.Flags().StringVarP(&outputFlag, "output", "o", "table", "Output format: table, json, or markdown")
	return cmd
}
