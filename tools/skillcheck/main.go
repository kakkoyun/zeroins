// Command skillcheck validates the skills/ directory against the repository's
// skill conventions. It is offline: no network, no cluster, no Helm.
//
// Usage: go run ./tools/skillcheck [repo-root]
//
// With no argument, the current working directory is used.
package main

import (
	"fmt"
	"os"

	"github.com/kakkoyun/zeroins/internal/skillcheck"
)

func main() {
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "skillcheck:", err)
		os.Exit(1)
	}
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	if err := skillcheck.Run(root, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
