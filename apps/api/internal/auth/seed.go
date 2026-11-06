package auth

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

const DemoQueueID = "10000000-0000-4000-8000-000000000001"

// SeedDemo requires operator-supplied credentials. Re-running preserves accounts,
// passwords and deliberately revoked permissions rather than resetting access.
func SeedDemo(ctx context.Context, database *gorm.DB, getenv func(string) string) error {
	type account struct{ ID, Name, Email, Role, Env string }
	accounts := []account{
		{"20000000-0000-4000-8000-000000000001", "Customer One", "customer1@example.test", "CUSTOMER", "SEED_CUSTOMER_PASSWORD"},
		{"20000000-0000-4000-8000-000000000002", "Customer Two", "customer2@example.test", "CUSTOMER", "SEED_CUSTOMER2_PASSWORD"},
		{"20000000-0000-4000-8000-000000000003", "Restaurant Host", "host@example.test", "HOST", "SEED_HOST_PASSWORD"},
		{"20000000-0000-4000-8000-000000000004", "Unassigned Host", "other-host@example.test", "HOST", "SEED_OTHER_HOST_PASSWORD"},
	}
	hashes := make([]string, len(accounts))
	for i, a := range accounts {
		hash, err := HashPassword(getenv(a.Env))
		if err != nil {
			return errors.New(a.Env + " must contain a password of 12–128 bytes")
		}
		hashes[i] = hash
	}
	return database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(732841001)").Error; err != nil {
			return err
		}
		if err := tx.Exec("INSERT INTO restaurants(id,name,timezone) VALUES (?,?,?) ON CONFLICT DO NOTHING", "30000000-0000-4000-8000-000000000001", "DineQueue Demo", "Asia/Bangkok").Error; err != nil {
			return err
		}
		if err := tx.Exec("INSERT INTO service_queues(id,restaurant_id,name,ticket_prefix) VALUES (?,?,?,?) ON CONFLICT DO NOTHING", DemoQueueID, "30000000-0000-4000-8000-000000000001", "Walk-in Dining", "A").Error; err != nil {
			return err
		}
		for i, a := range accounts {
			result := tx.Exec("INSERT INTO users(id,display_name,email,password_hash,role) VALUES (?,?,?,?,?) ON CONFLICT DO NOTHING", a.ID, a.Name, a.Email, hashes[i], a.Role)
			if result.Error != nil {
				return result.Error
			}
			var existing struct{ ID, Email, Role string }
			if err := tx.Table("users").Select("id,email,role").Where("id = ?", a.ID).Take(&existing).Error; err != nil {
				return errors.New("seed identity conflicts with an existing account")
			}
			if existing.Email != a.Email || existing.Role != a.Role {
				return errors.New("seed identity conflicts with an existing account")
			}
			if a.Role == "HOST" && a.Email == "host@example.test" && result.RowsAffected == 1 {
				if err := tx.Exec("INSERT INTO operator_queue_permissions(user_id,queue_id) VALUES (?,?) ON CONFLICT DO NOTHING", a.ID, DemoQueueID).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}
