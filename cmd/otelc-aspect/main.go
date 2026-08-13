package main

import (
	"fmt"
	"os"

	"github.com/kakkoyun/zeroins/internal/otelcaspect"
)

func main() {
	fmt.Fprintln(os.Stderr, "notice: otelc-aspect is deprecated; use `zeroins otelc lookup` instead.")
	os.Exit(otelcaspect.Run(os.Args[1:], os.Stdout, os.Stderr))
}
