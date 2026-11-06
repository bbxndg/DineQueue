package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dinequeue.local/api/internal/health"
	"dinequeue.local/api/internal/platform/config"
	"dinequeue.local/api/internal/platform/postgres"
	httptransport "dinequeue.local/api/internal/transport/http"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	app := httptransport.New(health.New(postgres.NewReadiness(pool)))
	listenError := make(chan error, 1)
	go func() { listenError <- app.Listen(fmt.Sprintf(":%d", cfg.Port)) }()
	select {
	case <-ctx.Done():
		return app.ShutdownWithTimeout(10 * time.Second)
	case err := <-listenError:
		if err != nil {
			return fmt.Errorf("HTTP listener stopped")
		}
		return nil
	}
}
