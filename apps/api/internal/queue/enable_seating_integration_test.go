//go:build integration

package queue

import (
	"context"
	"reflect"
	"testing"
	"time"

	"dinequeue.local/api/internal/auth"
	"github.com/google/uuid"
)

// S2-T-044, S2-T-043: guarded enable-seating activation (#54).

type preservedRow struct {
	ID             string
	Status         string
	IsReady        bool
	SequenceNumber int
	JoinedAt       time.Time
	Version        int64
	CalledAt       *time.Time
	CompletedAt    *time.Time
}

func entrySnapshot(t *testing.T, f fixture) []preservedRow {
	t.Helper()
	var rows []preservedRow
	if err := f.db.Raw("SELECT id,status,is_ready,sequence_number,joined_at,version,called_at,completed_at FROM queue_entries ORDER BY id").Scan(&rows).Error; err != nil {
		t.Fatal(err)
	}
	return rows
}

func mode(t *testing.T, f fixture, q string) string {
	t.Helper()
	var m string
	if err := f.db.Raw("SELECT operations_mode FROM service_queues WHERE id=?", q).Scan(&m).Error; err != nil {
		t.Fatal(err)
	}
	return m
}

func hashes(t *testing.T, f fixture) []string {
	t.Helper()
	var h []string
	if err := f.db.Raw("SELECT password_hash FROM users ORDER BY id").Scan(&h).Error; err != nil {
		t.Fatal(err)
	}
	return h
}

func TestF16EnableSeatingRefusesUnsafeActivation(t *testing.T) {
	ctx := context.Background()
	f := setup(t)
	var dbName string
	if err := f.db.Raw("SELECT current_database()").Scan(&dbName).Error; err != nil {
		t.Fatal(err)
	}
	// No configured floor yet.
	if err := EnableSeating(ctx, f.db, auth.DemoQueueID, dbName); err != ErrActivationFloor {
		t.Fatalf("no floor: %v", err)
	}
	provisionTestFloor(t, f)
	for _, bad := range []string{"", "wrong_test", dbName + "x", "DQ_GO_TEST"} {
		if err := EnableSeating(ctx, f.db, auth.DemoQueueID, bad); err == nil {
			t.Fatalf("confirmation %q accepted", bad)
		}
	}
	if err := EnableSeating(ctx, f.db, "not-a-uuid", dbName); err == nil {
		t.Fatal("malformed queue accepted")
	}
	if err := EnableSeating(ctx, f.db, uuid.NewString(), dbName); err != ErrNotFound {
		t.Fatalf("unknown queue %v", err)
	}
	// Legacy CALLED ticket blocks activation; nothing changes.
	called := joinAs(t, f)
	waitingEntry(t, f, auth.DemoQueueID, 90, time.Now().Add(time.Minute))
	if res := hostCall(t, f, "POST", hostPath(auth.DemoQueueID, "/call-next"), session(t, f, host), true); res.status != 200 {
		t.Fatalf("call %d", res.status)
	}
	before := entrySnapshot(t, f)
	if err := EnableSeating(ctx, f.db, auth.DemoQueueID, dbName); err != ErrActivationLegacyCalled {
		t.Fatalf("legacy CALLED: %v", err)
	}
	if mode(t, f, auth.DemoQueueID) != "MANUAL" || !reflect.DeepEqual(before, entrySnapshot(t, f)) {
		t.Fatal("refused activation changed data")
	}
	// An inactive zone-only floor is not a valid configured floor.
	if err := f.db.Exec("UPDATE floor_zones SET is_active=false").Error; err != nil {
		t.Fatal(err)
	}
	if res := hostCall(t, f, "POST", hostPath(auth.DemoQueueID, "/entries/"+called.body["id"].(string)+"/complete"), session(t, f, host), true); res.status != 200 {
		t.Fatalf("complete %d %v", res.status, res.body)
	}
	if err := EnableSeating(ctx, f.db, auth.DemoQueueID, dbName); err != ErrActivationFloor {
		t.Fatalf("inactive floor: %v", err)
	}
	if mode(t, f, auth.DemoQueueID) != "MANUAL" {
		t.Fatal("invalid floor activated")
	}
}

func TestF16EnableSeatingPreservesLegacyData(t *testing.T) {
	ctx := context.Background()
	f := setup(t)
	dbName := provisionTestFloor(t, f)
	// History: one completed manual ticket, plus WAITING ready / not-ready tickets.
	done := joinAs(t, f)
	hostToken := session(t, f, host)
	if res := hostCall(t, f, "POST", hostPath(auth.DemoQueueID, "/call-next"), hostToken, true); res.status != 200 {
		t.Fatalf("call %d", res.status)
	}
	if res := hostCall(t, f, "POST", hostPath(auth.DemoQueueID, "/entries/"+done.body["id"].(string)+"/complete"), hostToken, true); res.status != 200 {
		t.Fatalf("complete %d", res.status)
	}
	joinAs(t, f)
	joinNotReady(t, f)
	// A queue closed to joins can still be activated so existing tickets drain.
	if err := f.db.Exec("UPDATE service_queues SET is_active=false WHERE id=?", auth.DemoQueueID).Error; err != nil {
		t.Fatal(err)
	}
	before, beforeHashes := entrySnapshot(t, f), hashes(t, f)
	var counters int64
	f.db.Raw("SELECT coalesce(sum(last_number),0) FROM queue_day_counters").Scan(&counters)

	if err := EnableSeating(ctx, f.db, auth.DemoQueueID, dbName); err != nil {
		t.Fatal(err)
	}
	if mode(t, f, auth.DemoQueueID) != "SEATING" {
		t.Fatal("not activated")
	}
	// Rerun is an idempotent no-op.
	if err := EnableSeating(ctx, f.db, auth.DemoQueueID, dbName); err != nil {
		t.Fatalf("rerun %v", err)
	}
	if !reflect.DeepEqual(before, entrySnapshot(t, f)) || !reflect.DeepEqual(beforeHashes, hashes(t, f)) {
		t.Fatal("activation changed tickets or credentials")
	}
	var after int64
	f.db.Raw("SELECT coalesce(sum(last_number),0) FROM queue_day_counters").Scan(&after)
	if after != counters {
		t.Fatal("counter changed")
	}
	if n := countSQL(t, f, "SELECT count(*) FROM seating_assignments"); n != 0 {
		t.Fatal("activation fabricated an assignment")
	}
	// S2-T-043: once SEATING, the legacy bodyless call is guarded.
	if res := hostCall(t, f, "POST", hostPath(auth.DemoQueueID, "/call-next"), hostToken, true); res.status != 409 {
		t.Fatalf("legacy call after activation %d %v", res.status, res.body)
	}
	// Other queues are untouched.
	if n := countSQL(t, f, "SELECT count(*) FROM service_queues WHERE id<>? AND operations_mode<>'MANUAL'", auth.DemoQueueID); n != 0 {
		t.Fatal("other queue activated")
	}
}
