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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := worker.Run(ctx, cfg); err != nil {
		applog.Err.Fatalf("reason: %v", err)
	}
}
