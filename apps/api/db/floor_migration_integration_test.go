//go:build integration

package db

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"

	"dinequeue.local/api/internal/platform/postgres"
	"github.com/google/uuid"
)

func TestFloorMigrationPreservesLegacyRows(t *testing.T) {
	ctx := context.Background()
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Fatal("isolated TEST_DATABASE_URL required")
	}
	root, err := postgres.Open(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	rootPool, _ := root.DB()
	defer rootPool.Close()
	name := "floor_upgrade_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := root.Exec("CREATE SCHEMA " + name).Error; err != nil {
		t.Fatal(err)
	}
	defer root.Exec("DROP SCHEMA " + name + " CASCADE")
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	query.Set("search_path", name)
	parsed.RawQuery = query.Encode()
	database, err := postgres.Open(ctx, parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	pool, _ := database.DB()
	defer pool.Close()
	if err := database.Exec("CREATE TABLE schema_migrations(version text PRIMARY KEY, checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT now())").Error; err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"0001_auth.sql", "0002_queue_entries.sql", "0003_customer_readiness.sql"} {
		contents, err := migrations.ReadFile("migrations/" + file)
		if err != nil {
			t.Fatal(err)
		}
		if err := database.Exec(string(contents)).Error; err != nil {
			t.Fatal(err)
		}
		if err := database.Exec("INSERT INTO schema_migrations(version,checksum) VALUES (?,?)", file, fmt.Sprintf("%x", sha256.Sum256(contents))).Error; err != nil {
			t.Fatal(err)
		}
	}
	restaurant, queue, host := uuid.NewString(), uuid.NewString(), uuid.NewString()
	if err := database.Exec("INSERT INTO restaurants(id,name) VALUES (?,'Legacy')", restaurant).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Exec("INSERT INTO service_queues(id,restaurant_id,name,ticket_prefix) VALUES (?,?,'Legacy','L')", queue, restaurant).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Exec("INSERT INTO users(id,display_name,email,password_hash,role) VALUES (?,'Legacy host','legacy-host@example.test','unchanged-hash','HOST')", host).Error; err != nil {
		t.Fatal(err)
	}
	for i, status := range []string{"WAITING", "CALLED", "COMPLETED"} {
		customer := uuid.NewString()
		if err := database.Exec("INSERT INTO users(id,display_name,email,password_hash,role) VALUES (?,'Legacy customer',?,'unchanged-hash','CUSTOMER')", customer, customer+"@example.test").Error; err != nil {
			t.Fatal(err)
		}
		var called, completed, actor, finisher any
		if status != "WAITING" {
			called = "2026-11-01T01:01:00Z"
			actor = host
		}
		if status == "COMPLETED" {
			completed = "2026-11-01T01:02:00Z"
			finisher = host
		}
		if err := database.Exec(`INSERT INTO queue_entries(id,queue_id,customer_id,service_date,sequence_number,party_size,status,joined_at,called_at,called_by,completed_at,completed_by,is_ready)
   VALUES (?,?,?,'2026-11-01',?,10,?,'2026-11-01T01:00:00Z',?,?,?,?,false)`, uuid.NewString(), queue, customer, i+1, status, called, actor, completed, finisher).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := database.Exec("INSERT INTO queue_day_counters VALUES (?,'2026-11-01',3)", queue).Error; err != nil {
		t.Fatal(err)
	}
	before := map[string]string{}
	for _, table := range []string{"users", "queue_entries", "queue_day_counters"} {
		var data string
		if err := database.Raw("SELECT jsonb_agg(to_jsonb(t) ORDER BY to_jsonb(t)::text)::text FROM " + table + " t").Scan(&data).Error; err != nil {
			t.Fatal(err)
		}
		before[table] = data
	}
	if err := Migrate(ctx, database); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, database); err != nil {
		t.Fatal(err)
	}
	for table, want := range before {
		var actual string
		projection := "to_jsonb(t)"
		if table == "queue_entries" {
			projection = "to_jsonb(t) - ARRAY['cancelled_at','cancelled_by','no_show_at','no_show_by','no_show_reason']"
		}
		if err := database.Raw("SELECT jsonb_agg(" + projection + " ORDER BY (" + projection + ")::text)::text FROM " + table + " t").Scan(&actual).Error; err != nil {
			t.Fatal(err)
		}
		if actual != want {
			t.Fatalf("migration changed legacy %s", table)
		}
	}
	var mode string
	database.Raw("SELECT operations_mode FROM service_queues WHERE id=?", queue).Scan(&mode)
	if mode != "MANUAL" {
		t.Fatalf("legacy mode %s", mode)
	}
	var versions int64
	database.Table("schema_migrations").Count(&versions)
	if versions != 5 {
		t.Fatalf("versions %d", versions)
	}
	var terminalFields int64
	if err := database.Raw("SELECT count(*) FROM queue_entries WHERE cancelled_at IS NOT NULL OR cancelled_by IS NOT NULL OR no_show_at IS NOT NULL OR no_show_by IS NOT NULL OR no_show_reason IS NOT NULL").Scan(&terminalFields).Error; err != nil {
		t.Fatal(err)
	}
	if terminalFields != 0 {
		t.Fatal("legacy terminal fields were populated")
	}

}
