//go:build integration

package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"

	schema "dinequeue.local/api/db"
	"dinequeue.local/api/internal/auth"
	"github.com/google/uuid"
)

func provisionTestFloor(t *testing.T, f fixture) string {
	t.Helper()
	var name string
	if err := f.db.Raw("SELECT current_database()").Scan(&name).Error; err != nil {
		t.Fatal(err)
	}
	if err := ProvisionFloor(context.Background(), f.db, auth.DemoQueueID, name); err != nil {
		t.Fatal(err)
	}
	return name
}
func floorRead(t *testing.T, f fixture) FloorSnapshot {
	t.Helper()
	result, err := f.service.Floor(context.Background(), auth.User{ID: host, Role: "HOST"}, auth.DemoQueueID)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestFloorInventoryAndReadOnlySnapshot(t *testing.T) {
	f := setup(t)
	provisionTestFloor(t, f)
	before := floorRead(t, f)
	if before.OperationsMode != "MANUAL" || before.HoldTTLSeconds != 300 || before.LayoutVersion != 2 || len(before.Zones) != 3 {
		t.Fatalf("snapshot: %+v", before)
	}
	want := map[string]int{"T1": 2, "T2": 4, "T3": 8, "T4": 6, "T5": 4, "T6": 2, "T7": 8}
	codes := map[string]string{}
	edges := []string{}
	for _, zone := range before.Zones {
		for _, table := range zone.Tables {
			if want[table.Code] != table.Capacity || table.Status != "AVAILABLE" || table.Version != 1 {
				t.Fatalf("table: %+v", table)
			}
			codes[table.ID] = table.Code
			delete(want, table.Code)
		}
	}
	for _, zone := range before.Zones {
		for _, edge := range zone.Connections {
			pair := []string{codes[edge.TableIDs[0]], codes[edge.TableIDs[1]]}
			if pair[0] > pair[1] {
				pair[0], pair[1] = pair[1], pair[0]
			}
			edges = append(edges, strings.Join(pair, "+"))
		}
	}
	if len(want) != 0 || len(edges) != 4 {
		t.Fatalf("missing %v edges %v", want, edges)
	}
	for _, edge := range []string{"T1+T2", "T2+T3", "T4+T5", "T5+T6"} {
		if !strings.Contains(strings.Join(edges, ","), edge) {
			t.Fatalf("missing %s", edge)
		}
	}
	r := hostCall(t, f, "GET", hostPath(auth.DemoQueueID, "/floor"), session(t, f, host), false)
	if r.status != 200 {
		t.Fatalf("HTTP %d %v", r.status, r.body)
	}
	encoded, _ := json.Marshal(r.body)
	for _, key := range []string{"customerId", "email", "password", "token", "approvedBy", "entryId"} {
		if strings.Contains(string(encoded), `"`+key+`"`) {
			t.Fatalf("private field %s", key)
		}
	}
	after := floorRead(t, f)
	after.ServerNow = before.ServerNow
	if !reflect.DeepEqual(before, after) {
		t.Fatal("GET changed persisted inventory")
	}
	// Shared restaurant queue: exact grant still required, shared states contain no assignment identity.
	q2 := uuid.NewString()
	if err := f.db.Exec("INSERT INTO service_queues(id,restaurant_id,name,ticket_prefix) VALUES (?,?,'Q2','B')", q2, before.RestaurantID).Error; err != nil {
		t.Fatal(err)
	}
	if r := hostCall(t, f, "GET", hostPath(q2, "/floor"), session(t, f, host), false); r.status != 403 {
		t.Fatalf("same restaurant grant bypass %d", r.status)
	}
	if err := f.db.Exec("INSERT INTO operator_queue_permissions(user_id,queue_id) VALUES (?,?)", host, q2).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Exec("UPDATE dining_tables SET status='OCCUPIED',version=version+1 WHERE code='T4'").Error; err != nil {
		t.Fatal(err)
	}
	r = hostCall(t, f, "GET", hostPath(q2, "/floor"), session(t, f, host), false)
	if r.status != 200 || len(r.body["assignments"].([]any)) != 0 || !strings.Contains(fmt.Sprint(r.body), "OCCUPIED") {
		t.Fatalf("shared snapshot %v", r)
	}
}

func TestFloorProvisionPreservesLiveDataAndRejectsConflicts(t *testing.T) {
	f := setup(t)
	name := provisionTestFloor(t, f)
	// These states are inventory fixtures, not a claim that seating lifecycle exists.
	if err := f.db.Exec("UPDATE dining_tables SET status='DIRTY',version=7 WHERE code='T4'").Error; err != nil {
		t.Fatal(err)
	}
	before := floorRead(t, f)
	var hashesBefore, hashesAfter []string
	f.db.Raw("SELECT password_hash FROM users ORDER BY id").Scan(&hashesBefore)
	if err := ProvisionFloor(context.Background(), f.db, auth.DemoQueueID, name); err != nil {
		t.Fatal(err)
	}
	f.db.Raw("SELECT password_hash FROM users ORDER BY id").Scan(&hashesAfter)
	after := floorRead(t, f)
	after.ServerNow = before.ServerNow
	if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(hashesBefore, hashesAfter) {
		t.Fatal("rerun changed live data")
	}
	if err := ProvisionFloor(context.Background(), f.db, auth.DemoQueueID, "wrong_demo"); err == nil {
		t.Fatal("wrong confirmation accepted")
	}
	if err := ProvisionFloor(context.Background(), f.db, auth.DemoQueueID, "production"); err == nil {
		t.Fatal("general database accepted")
	}
	if err := f.db.Exec("UPDATE dining_tables SET capacity=5 WHERE code='T4'").Error; err != nil {
		t.Fatal(err)
	}
	if err := ProvisionFloor(context.Background(), f.db, auth.DemoQueueID, name); !errors.Is(err, ErrFloorConfiguration) {
		t.Fatalf("configuration conflict: %v", err)
	}
	var capacity int
	f.db.Raw("SELECT capacity FROM dining_tables WHERE code='T4'").Scan(&capacity)
	if capacity != 5 {
		t.Fatal("conflicting config silently overwritten")
	}
	if err := f.db.Exec("UPDATE dining_tables SET capacity=6 WHERE code='T4'").Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Exec("UPDATE table_connections SET is_active=false").Error; err != nil {
		t.Fatal(err)
	}
	if err := ProvisionFloor(context.Background(), f.db, auth.DemoQueueID, name); !errors.Is(err, ErrFloorConfiguration) {
		t.Fatalf("inactive approved graph was silently restored: %v", err)
	}
	var activeEdges int64
	if err := f.db.Table("table_connections").Where("is_active").Count(&activeEdges).Error; err != nil {
		t.Fatal(err)
	}
	if activeEdges != 0 {
		t.Fatal("failed provision rewrote graph")
	}
}

func TestFloorConcurrentProvisionAndConstraints(t *testing.T) {
	f := setup(t)
	var name string
	f.db.Raw("SELECT current_database()").Scan(&name)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- ProvisionFloor(context.Background(), f.db, auth.DemoQueueID, name) }()
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	if count(t, f, "dining_tables") != 7 || count(t, f, "table_connections") != 4 {
		t.Fatal("duplicate fixture")
	}
	for _, change := range []string{"capacity=0", "capacity=9", "position_x='NaN'::float8", "position_y='Infinity'::float8", "position_x=-1", "position_y=101", "version=0", "status='FAKE'"} {
		if err := f.db.Exec("UPDATE dining_tables SET " + change + " WHERE code='T1'").Error; err == nil {
			t.Fatalf("constraint allowed %s", change)
		}
	}
	// Composite FKs reject a cross-zone edge even with canonical endpoint ordering.
	var tables []struct{ ID, ZoneID, RestaurantID string }
	f.db.Raw("SELECT id,zone_id,restaurant_id FROM dining_tables WHERE code IN ('T4','T7') ORDER BY id").Scan(&tables)
	if err := f.db.Exec("INSERT INTO table_connections(restaurant_id,zone_id,table_low_id,table_high_id) VALUES (?,?,?,?)", tables[0].RestaurantID, tables[0].ZoneID, tables[0].ID, tables[1].ID).Error; err == nil {
		t.Fatal("cross-zone edge accepted")
	}
	table := tables[0]
	if err := f.db.Exec("INSERT INTO table_connections(restaurant_id,zone_id,table_low_id,table_high_id) VALUES (?,?,?,?)", table.RestaurantID, table.ZoneID, table.ID, table.ID).Error; err == nil {
		t.Fatal("self edge accepted")
	}
	if err := schema.Migrate(context.Background(), f.db); err != nil {
		t.Fatalf("migration rerun: %v", err)
	}
}
