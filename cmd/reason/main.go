package main

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/straubt1/redpanda-build-exercise/internal/applog"
	"github.com/straubt1/redpanda-build-exercise/internal/config"
	"github.com/straubt1/redpanda-build-exercise/internal/worker"
)

func main() {
	cfg, err := config.FromEnv()
	if err != nil {
		applog.Err.Fatalf("config: %v", err)
	}

	// Cancel on Ctrl+C (SIGINT) and docker stop (SIGTERM) so Poll exits cleanly.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := worker.Run(ctx, cfg); err != nil {
		applog.Err.Fatalf("reason: %v", err)
	}
}
