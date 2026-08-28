package main

import (
	"context"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/straubt1/redpanda-build-exercise/internal/applog"
	"github.com/straubt1/redpanda-build-exercise/internal/config"
	"github.com/straubt1/redpanda-build-exercise/internal/serve"
	"github.com/straubt1/redpanda-build-exercise/internal/store"
)

func main() {
	cfg, err := config.ServeFromEnv()
	if err != nil {
		applog.Err.Fatalf("config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := store.Connect(ctx, cfg.PostgresDSN)
	if err != nil {
		applog.Err.Fatalf("postgres: %v", err)
	}
	defer db.Close()

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           serve.New(db, cfg.ListCap).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		applog.Info.Printf("serve listening %s list_cap=%d", cfg.HTTPAddr, cfg.ListCap)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			applog.Err.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdown)
}
