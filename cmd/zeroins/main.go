package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/kakkoyun/zeroins/internal/zeroins"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(zeroins.Main(ctx, os.Args[1:], zeroins.DefaultDependencies()))
}
