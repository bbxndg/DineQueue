package auth

import (
	"context"
	"errors"
	"regexp"

	"gorm.io/gorm"
)

// ProvisionDemo is a separate operator-only CLI path, never an HTTP endpoint.
// It only targets an explicitly confirmed demo/UAT database, preserves existing
// identities/access, and does not enable the ordinary production seed command.
func ProvisionDemo(ctx context.Context, database *gorm.DB, getenv func(string) string) error {
	name := getenv("CONFIRM_DEMO_DATABASE")
	if getenv("ENABLE_DEMO_PROVISION") != "true" || !regexp.MustCompile(`^[a-z][a-z0-9_]{0,51}_(demo|uat)$`).MatchString(name) {
		return errors.New("demo provisioning requires explicit opt-in and a confirmed demo/UAT database")
	}
	passwords := map[string]string{
		"SEED_CUSTOMER_PASSWORD":   getenv("DEMO_CUSTOMER_PASSWORD"),
		"SEED_CUSTOMER2_PASSWORD":  getenv("DEMO_CUSTOMER2_PASSWORD"),
		"SEED_HOST_PASSWORD":       getenv("DEMO_HOST_PASSWORD"),
		"SEED_OTHER_HOST_PASSWORD": getenv("DEMO_OTHER_HOST_PASSWORD"),
	}
	return database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var actual string
		if err := tx.Raw("SELECT current_database()").Scan(&actual).Error; err != nil || actual != name {
			return errors.New("demo database confirmation does not match the connected database")
		}
		if err := tx.Exec("SELECT pg_advisory_xact_lock(732841001)").Error; err != nil {
			return errors.New("cannot lock demo provisioning")
		}
		var conflicts int64
		if err := tx.Raw(`SELECT
			(SELECT count(*) FROM restaurants WHERE id = '30000000-0000-4000-8000-000000000001'
			AND (name <> 'DineQueue Demo' OR timezone <> 'Asia/Bangkok')) +
			(SELECT count(*) FROM service_queues WHERE id = ? AND
			(restaurant_id <> '30000000-0000-4000-8000-000000000001' OR name <> 'Walk-in Dining' OR ticket_prefix <> 'A'))`, DemoQueueID).Scan(&conflicts).Error; err != nil || conflicts != 0 {
			return errors.New("demo restaurant or queue conflicts with existing data")
		}
		return SeedDemo(ctx, tx, func(key string) string { return passwords[key] })
	})
}
