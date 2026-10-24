package main

import (
	"context"
	"github.com/dpologdvinity/flaky/internal/flaky"
	"os"
	"os/signal"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(flaky.Main(ctx, os.Args[1:], os.Stdout, os.Stderr))
}
