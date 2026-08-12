package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/kakkoyun/zeroins/internal/kubectlprofiler"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(kubectlprofiler.Main(ctx, os.Args[1:], kubectlprofiler.DefaultDependencies()))
}
