package postgres

import (
	"context"
	"strings"
	"testing"
)

func TestOpenIsLazyWithBoundedPool(t *testing.T) {
	database, err := Open(context.Background(), "postgres://local:fixture-only@127.0.0.1:1/unreachable?sslmode=disable")
	if err != nil {
		t.Fatal("opening configuration should not require a reachable database")
	}
	pool, err := database.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	stats := pool.Stats()
	if stats.MaxOpenConnections != 10 || stats.OpenConnections != 0 {
		t.Fatalf("unexpected initial pool: max=%d open=%d", stats.MaxOpenConnections, stats.OpenConnections)
	}
}

func TestOpenRejectsMalformedDSNWithoutSecrets(t *testing.T) {
	_, err := Open(context.Background(), "postgres://local:do-not-expose@localhost:bad/database")
	if err == nil {
		t.Fatal("malformed database configuration accepted")
	}
	if strings.Contains(err.Error(), "do-not-expose") || strings.Contains(err.Error(), "postgres://") {
		t.Fatal("database configuration error leaked credentials")
	}
}
