// Command render-profiler-values emits the exact Helm values used by kubectl-profiler.
package main

import (
	"fmt"
	"os"

	"github.com/kakkoyun/zeroins/internal/kubectlprofiler"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: render-profiler-values host:port")
		os.Exit(1)
	}
	values, err := kubectlprofiler.RenderValues(os.Args[1], true)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if _, err := os.Stdout.Write(values); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
