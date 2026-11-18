//go:build integration

package auth

import (
	"context"
	"dinequeue.local/api/internal/platform/postgres"
	"github.com/google/uuid"
	"net/url"
	"os"
	"strings"
	"testing"
)

func TestProvisionDemoRequiresExplicitDatabaseAndPreservesExistingAccess(t *testing.T) {
	f := provisionFixture(t)
	var name string
	if err := f.db.Raw("SELECT current_database()").Scan(&name).Error; err != nil {
		t.Fatal(err)
	}
	env := map[string]string{
		"ENABLE_DEMO_PROVISION": "true", "CONFIRM_DEMO_DATABASE": name,
		"DEMO_CUSTOMER_PASSWORD": "demo-customer-test-only", "DEMO_CUSTOMER2_PASSWORD": "demo-second-test-only",
		"DEMO_HOST_PASSWORD": "demo-host-test-only", "DEMO_OTHER_HOST_PASSWORD": "demo-other-test-only",
	}
	getenv := func(key string) string { return env[key] }
	for _, key := range []string{"ENABLE_DEMO_PROVISION", "CONFIRM_DEMO_DATABASE", "DEMO_HOST_PASSWORD"} {
		value := env[key]
		env[key] = ""
		if err := ProvisionDemo(context.Background(), f.db, getenv); err == nil {
			t.Fatalf("missing %s accepted", key)
		}
		env[key] = value
	}
	env["CONFIRM_DEMO_DATABASE"] = "other_demo"
	if err := ProvisionDemo(context.Background(), f.db, getenv); err == nil {
		t.Fatal("wrong database accepted")
	}
	env["CONFIRM_DEMO_DATABASE"] = name
	var before string
	f.db.Raw("SELECT password_hash FROM users WHERE email = 'host@example.test'").Scan(&before)
	f.db.Exec("UPDATE users SET is_active = false WHERE email = 'host@example.test'")
	f.db.Exec("DELETE FROM operator_queue_permissions")
	if err := ProvisionDemo(context.Background(), f.db, getenv); err != nil {
		t.Fatal(err)
	}
	var after string
	var active bool
	var grants int64
	f.db.Raw("SELECT password_hash FROM users WHERE email = 'host@example.test'").Scan(&after)
	f.db.Raw("SELECT is_active FROM users WHERE email = 'host@example.test'").Scan(&active)
	f.db.Table("operator_queue_permissions").Count(&grants)
	if before != after || active || grants != 0 {
		t.Fatal("provisioning changed existing password, active flag or revoked grant")
	}
}

func TestProvisionDemoFreshAccountsAndConflictRollback(t *testing.T) {
	f := provisionFixture(t)
	f.db.Exec("DELETE FROM operator_queue_permissions")
	f.db.Exec("DELETE FROM users")
	f.db.Exec("DELETE FROM service_queues")
	f.db.Exec("DELETE FROM restaurants")
	var name string
	f.db.Raw("SELECT current_database()").Scan(&name)
	env := map[string]string{"ENABLE_DEMO_PROVISION": "true", "CONFIRM_DEMO_DATABASE": name}
	for _, key := range []string{"DEMO_CUSTOMER_PASSWORD", "DEMO_CUSTOMER2_PASSWORD", "DEMO_HOST_PASSWORD", "DEMO_OTHER_HOST_PASSWORD"} {
		env[key] = "fresh-demo-test-password"
	}
	getenv := func(key string) string { return env[key] }
	if err := ProvisionDemo(context.Background(), f.db, getenv); err != nil {
		t.Fatal(err)
	}
	var count int64
	f.db.Table("users").Count(&count)
	if count != 4 {
		t.Fatal("expected four demo accounts")
	}
	var hash string
	f.db.Raw("SELECT password_hash FROM users WHERE email='customer1@example.test'").Scan(&hash)
	if !VerifyPassword(hash, env["DEMO_CUSTOMER_PASSWORD"]) {
		t.Fatal("provisioned password does not verify")
	}
	f.db.Exec("UPDATE service_queues SET ticket_prefix='B' WHERE id = ?", DemoQueueID)
	if err := ProvisionDemo(context.Background(), f.db, getenv); err == nil {
		t.Fatal("conflicting demo queue metadata accepted")
	}
	f.db.Exec("DELETE FROM operator_queue_permissions")
	f.db.Exec("DELETE FROM users")
	f.db.Exec("DELETE FROM service_queues")
	f.db.Exec("DELETE FROM restaurants")
	f.db.Exec("INSERT INTO users(id,display_name,email,password_hash,role) VALUES ('90000000-0000-4000-8000-000000000001','Conflict','customer2@example.test','unused','CUSTOMER')")
	if err := ProvisionDemo(context.Background(), f.db, getenv); err == nil {
		t.Fatal("identity conflict accepted")
	}
	f.db.Table("users").Count(&count)
	if count != 1 {
		t.Fatal("partial accounts committed after identity conflict")
	}
	f.db.Table("restaurants").Count(&count)
	if count != 0 {
		t.Fatal("partial restaurant committed after identity conflict")
	}
}

// The provisioning guard deliberately refuses ordinary databases. Use a fresh,
// explicitly named demo database instead of relaxing it for the CI database.
func provisionFixture(t *testing.T) fixture {
	t.Helper()
	raw := os.Getenv("TEST_DATABASE_URL")
	root, err := postgres.Open(context.Background(), raw)
	if err != nil {
		t.Fatal("cannot open isolated test database")
	}
	pool, _ := root.DB()
	name := "ep5_" + strings.ReplaceAll(uuid.NewString(), "-", "") + "_demo"
	if err := root.Exec("CREATE DATABASE " + name).Error; err != nil {
		t.Fatal("cannot create isolated provisioning database")
	}
	t.Cleanup(func() { root.Exec("DROP DATABASE " + name); pool.Close() })
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal("invalid test database URL")
	}
	parsed.Path = "/" + name
	t.Setenv("TEST_DATABASE_URL", parsed.String())
	return setup(t)
}
