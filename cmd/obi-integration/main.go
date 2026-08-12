package main

import (
	"os"

	"github.com/kakkoyun/zeroins/internal/obiintegration"
)

func main() {
	os.Exit(obiintegration.Run(os.Args[1:], os.Stdout, os.Stderr))
}
