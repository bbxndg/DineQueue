// Package db owns explicit, versioned schema changes. It is never called by API startup.
package db

import (
	"context"
	"crypto/sha256"
	"embed"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

//go:embed migrations/*.sql
var migrations embed.FS

func Migrate(ctx context.Context, database *gorm.DB) error {
	return database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(732841001)").Error; err != nil {
			return err
		}
		if err := tx.Exec("CREATE TABLE IF NOT EXISTS schema_migrations (version text PRIMARY KEY, checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT now())").Error; err != nil {
			return err
		}
		files, err := migrations.ReadDir("migrations")
		if err != nil {
			return err
		}
		for _, file := range files {
			if !strings.HasSuffix(file.Name(), ".sql") {
				continue
			}
			sql, err := migrations.ReadFile("migrations/" + file.Name())
			if err != nil {
				return err
			}
			checksum := fmt.Sprintf("%x", sha256.Sum256(sql))
			var rows []struct{ Checksum string }
			if err := tx.Raw("SELECT checksum FROM schema_migrations WHERE version = ?", file.Name()).Scan(&rows).Error; err != nil {
				return err
			}
			if len(rows) > 0 {
				if rows[0].Checksum != checksum {
					return fmt.Errorf("migration checksum mismatch: %s", file.Name())
				}
				continue
			}
			if err := tx.Exec(string(sql)).Error; err != nil {
				return err
			}
			if err := tx.Exec("INSERT INTO schema_migrations(version,checksum) VALUES (?,?)", file.Name(), checksum).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
