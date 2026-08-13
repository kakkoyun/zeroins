package main

import (
	"fmt"
	"os"

	"github.com/kakkoyun/zeroins/internal/obiintegration"
)

func main() {
	fmt.Fprintln(os.Stderr, "notice: obi-integration is deprecated; use `zeroins obi lookup` instead.")
	os.Exit(obiintegration.Run(os.Args[1:], os.Stdout, os.Stderr))
}
