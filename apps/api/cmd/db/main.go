package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"time"

	schema "dinequeue.local/api/db"
	"dinequeue.local/api/internal/auth"
	"dinequeue.local/api/internal/platform/config"
	"dinequeue.local/api/internal/platform/postgres"
	"dinequeue.local/api/internal/queue"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) < 2 || (os.Args[1] != "migrate" && os.Args[1] != "seed" && os.Args[1] != "provision-demo" && os.Args[1] != "provision-floor" && os.Args[1] != "enable-seating") {
		return errors.New("usage: db migrate|seed|provision-demo|provision-floor|enable-seating --queue-id=<uuid> --confirm-database=<database>")
	}
	flags := flag.NewFlagSet(os.Args[1], flag.ContinueOnError)
	queueID := flags.String("queue-id", "", "queue UUID")
	confirmedDatabase := flags.String("confirm-database", "", "exact synthetic database name")
	confirmProduction := flags.Bool("i-understand-production", false, "enable-seating only: required when APP_ENV=production")
	if os.Args[1] == "provision-floor" || os.Args[1] == "enable-seating" {
		if err := flags.Parse(os.Args[2:]); err != nil {
			return err
		}
		if flags.NArg() != 0 || *queueID == "" || *confirmedDatabase == "" {
			return errors.New("queue-id and confirm-database are required")
		}
		if *confirmProduction && os.Args[1] != "enable-seating" {
			return errors.New("--i-understand-production applies only to enable-seating")
		}
	} else if len(os.Args) != 2 {
		return errors.New("unexpected database command arguments")
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
	} else if os.Args[1] == "provision-floor" {
		err = queue.ProvisionFloor(ctx, database, *queueID, *confirmedDatabase)
	} else if os.Args[1] == "enable-seating" {
		// Explicit, guarded activation; never part of API startup.
		var result queue.ActivationResult
		result, err = queue.EnableSeating(ctx, database, *queueID, *confirmedDatabase, queue.ActivationOptions{Environment: cfg.Environment, ConfirmProduction: *confirmProduction})
		if err == nil {
			log.Printf("enable-seating: database %s queue %q (%s) mode %s -> %s", result.Database, result.QueueName, result.QueueID, result.ModeBefore, result.ModeAfter)
		}
	} else if os.Args[1] == "provision-demo" {
		err = auth.ProvisionDemo(ctx, database, os.Getenv)
	} else {
		err = auth.SeedDemo(ctx, database, os.Getenv)
	}
	// Fixed activation refusals carry no SQL or secrets; tell the operator why.
	for _, refusal := range []error{queue.ErrActivationConfirmation, queue.ErrActivationProductionConsent, queue.ErrActivationFloor, queue.ErrActivationLegacyCalled} {
		if errors.Is(err, refusal) {
			return refusal
		}
	}
	// Driver errors may include SQL parameters or credentials. Never print them here.
	if err != nil {
		return errors.New("database command failed; verify configuration, schema and seed password requirements")
	}
	log.Print("database command completed")
	return nil
}
