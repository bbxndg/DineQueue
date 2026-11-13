package main

import (
	"context"
	"errors"
	"log"
	"os"
	"time"

	schema "dinequeue.local/api/db"
	"dinequeue.local/api/internal/auth"
	"dinequeue.local/api/internal/platform/config"
	"dinequeue.local/api/internal/platform/postgres"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) != 2 || (os.Args[1] != "migrate" && os.Args[1] != "seed") {
		return errors.New("usage: go run ./cmd/db migrate|seed")
	}
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	if os.Args[1] == "seed" && cfg.Environment == "production" {
		return errors.New("demo seed is disabled in production")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	database, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	pool, err := database.DB()
	if err != nil {
		return errors.New("cannot access database")
	}
	defer pool.Close()
	if os.Args[1] == "migrate" {
		err = schema.Migrate(ctx, database)
	} else {
		err = auth.SeedDemo(ctx, database, os.Getenv)
	}
	// Driver errors may include SQL parameters or credentials. Never print them here.
	if err != nil {
		return errors.New("database command failed; verify configuration, schema and seed password requirements")
	}
	log.Print("database command completed")
	return nil
}
