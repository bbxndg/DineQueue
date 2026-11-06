//go:build integration

package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"gorm.io/gorm"
)

func testDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("TEST_DATABASE_URL is required for integration tests; use an isolated database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	database, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatal("ORM initialization failed")
	}
	pool, err := database.DB()
	if err != nil {
		t.Fatal("cannot access underlying pool")
	}
	t.Cleanup(func() { pool.Close() })
	return database
}

func TestPostgresReadiness(t *testing.T) {
	database := testDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := NewReadiness(database).Ping(ctx); err != nil {
		t.Fatal("database readiness check failed")
	}
	cancel()
	if err := NewReadiness(database).Ping(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal("ORM readiness did not honor context cancellation")
	}
}

func TestOpenDoesNotCreateApplicationTables(t *testing.T) {
	database := testDatabase(t)
	var count int64
	err := database.Raw("SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_type = 'BASE TABLE'").Scan(&count).Error
	if err != nil {
		t.Fatal("cannot inspect isolated database")
	}
	if count != 0 {
		t.Fatal("foundation integration requires an empty database and must not create application tables")
	}
}

func TestGORMTransactionCommitAndRollback(t *testing.T) {
	database := testDatabase(t)
	type probeRow struct {
		ID    int64
		Label string
	}
	wantRollback := errors.New("intentional fixture rollback")
	err := database.Connection(func(connection *gorm.DB) error {
		// A temporary table on one connection exercises ORM writes without domain migrations.
		if err := connection.Exec("CREATE TEMP TABLE foundation_gorm_probe (id bigserial PRIMARY KEY, label text NOT NULL)").Error; err != nil {
			return err
		}
		defer connection.Exec("DROP TABLE foundation_gorm_probe")
		err := connection.Transaction(func(tx *gorm.DB) error {
			if err := tx.Table("foundation_gorm_probe").Create(&probeRow{Label: "rollback"}).Error; err != nil {
				return err
			}
			return wantRollback
		})
		if !errors.Is(err, wantRollback) {
			return errors.New("expected fixture transaction rollback")
		}
		var count int64
		if err := connection.Table("foundation_gorm_probe").Count(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			return errors.New("rolled-back ORM insert remained")
		}
		if err := connection.Transaction(func(tx *gorm.DB) error {
			return tx.Table("foundation_gorm_probe").Create(&probeRow{Label: "commit"}).Error
		}); err != nil {
			return err
		}
		if err := connection.Table("foundation_gorm_probe").Count(&count).Error; err != nil {
			return err
		}
		if count != 1 {
			return errors.New("committed ORM insert missing")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ORM transaction fixture failed: %v", err)
	}
}
