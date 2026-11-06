//go:build integration

package postgres

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestPostgresReadiness(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("TEST_DATABASE_URL is required for integration tests; use an isolated database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatal("pool initialization failed")
	}
	defer pool.Close()
	if err := NewReadiness(pool).Ping(ctx); err != nil {
		t.Fatal("database readiness check failed")
	}
}
