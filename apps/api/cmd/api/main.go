package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dinequeue.local/api/internal/auth"
	"dinequeue.local/api/internal/health"
	"dinequeue.local/api/internal/platform/config"
	"dinequeue.local/api/internal/platform/postgres"
	"dinequeue.local/api/internal/queue"
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
	database, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	pool, err := database.DB()
	if err != nil {
		return fmt.Errorf("cannot access PostgreSQL pool")
	}
	defer pool.Close()
	app := httptransport.New(health.New(postgres.NewReadiness(database)))
	authService, err := auth.NewService(auth.NewRepository(database))
	if err != nil {
		return fmt.Errorf("cannot initialize authentication")
	}
	authHTTP := auth.NewHTTP(authService, cfg.AppOrigin)
	authHTTP.Register(app)
	queue.NewHTTP(queue.NewService(queue.NewRepository(database)), authHTTP).Register(app)
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
