package main

import (
	"os"

	"github.com/kakkoyun/zeroins/internal/otelcaspect"
)

func main() {
	os.Exit(otelcaspect.Run(os.Args[1:], os.Stdout, os.Stderr))
}
