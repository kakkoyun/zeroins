package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/kakkoyun/zeroins/internal/kubectlobi"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(kubectlobi.Main(ctx, os.Args[1:], kubectlobi.DefaultDependencies()))
}
