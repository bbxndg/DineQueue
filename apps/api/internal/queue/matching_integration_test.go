//go:build integration

package queue

import (
	"context"
	"dinequeue.local/api/internal/auth"
	"encoding/json"
	"testing"
	"time"
)

func TestMatchingPreviewFIFOAndReadOnly(t *testing.T) {
	f := setup(t)
	provisionTestFloor(t, f)
	floor := floorRead(t, f)
	var ids []string
	for _, z := range floor.Zones {
		for _, table := range z.Tables {
			if table.Code == "T4" || table.Code == "T5" {
				ids = append(ids, table.ID)
			}
		}
	}
	first := waitingEntry(t, f, auth.DemoQueueID, 1, time.Now().Add(-time.Minute))
	second := waitingEntry(t, f, auth.DemoQueueID, 2, time.Now())
	if err := f.db.Exec("UPDATE queue_entries SET party_size=10,is_ready=(id=?)", second).Error; err != nil {
		t.Fatal(err)
	}
	baseline := floorRead(t, f)
	baseline.ServerNow = floor.ServerNow
	before, _ := json.Marshal(baseline)
	p, err := f.service.Matching(context.Background(), auth.User{ID: host, Role: "HOST"}, auth.DemoQueueID, ids, "")
	if err != nil || p.Entry == nil || p.Entry.ID != second || len(p.Candidates) != 1 || len(p.Skipped) != 1 || p.Skipped[0].ReasonCode != "NOT_READY" {
		t.Fatalf("preview %+v %v", p, err)
	}
	if err := f.db.Exec("UPDATE queue_entries SET is_ready=true WHERE id=?", first).Error; err != nil {
		t.Fatal(err)
	}
	p, err = f.service.Matching(context.Background(), auth.User{ID: host, Role: "HOST"}, auth.DemoQueueID, ids, "")
	if err != nil || p.Entry == nil || p.Entry.ID != first {
		t.Fatalf("original priority %+v %v", p, err)
	}
	after := floorRead(t, f)
	after.ServerNow = floor.ServerNow
	a, _ := json.Marshal(after)
	if string(a) != string(before) {
		t.Fatal("preview changed floor")
	}
	token := session(t, f, host)
	for _, query := range []string{"?entryId=" + first, "?tableId=" + ids[0] + "&tableId=" + ids[1]} {
		r := hostCall(t, f, "GET", hostPath(auth.DemoQueueID, "/matching"+query), token, false)
		if r.status != 200 {
			t.Fatalf("HTTP %d %v", r.status, r.body)
		}
	}
	for _, query := range []string{"", "?tableId=" + ids[0] + "&tableId=" + ids[0], "?entryId=" + first + "&tableId=" + ids[0], "?capacity=100"} {
		r := hostCall(t, f, "GET", hostPath(auth.DemoQueueID, "/matching"+query), token, false)
		if r.status != 400 {
			t.Fatalf("query %s: %d", query, r.status)
		}
	}
}

func TestMatchingAccessAndAvailability(t *testing.T) {
	f := setup(t)
	provisionTestFloor(t, f)
	var id string
	if err := f.db.Raw("SELECT id FROM dining_tables WHERE code='T7'").Scan(&id).Error; err != nil {
		t.Fatal(err)
	}
	path := hostPath(auth.DemoQueueID, "/matching?tableId="+id)
	for _, tc := range []struct {
		user   string
		status int
	}{{"", 401}, {customer1, 403}, {otherHost, 403}, {host, 200}} {
		token := ""
		if tc.user != "" {
			token = session(t, f, tc.user)
		}
		r := hostCall(t, f, "GET", path, token, false)
		if r.status != tc.status {
			t.Fatalf("actor %s: %d", tc.user, r.status)
		}
		if tc.status == 200 && r.body["availabilityReason"] != "QUEUE_EMPTY" {
			t.Fatal(r.body)
		}
	}
	entry := waitingEntry(t, f, auth.DemoQueueID, 1, time.Now())
	if err := f.db.Exec("UPDATE queue_entries SET party_size=10,is_ready=true WHERE id=?", entry).Error; err != nil {
		t.Fatal(err)
	}
	r := hostCall(t, f, "GET", path, session(t, f, host), false)
	if r.body["availabilityReason"] != "NO_FITTING_PARTY" {
		t.Fatal(r.body)
	}
	for _, state := range []string{"HELD", "OCCUPIED", "DIRTY", "CLEANING", "OUT_OF_SERVICE"} {
		if err := f.db.Exec("UPDATE dining_tables SET status=? WHERE id=?", state, id).Error; err != nil {
			t.Fatal(err)
		}
		r := hostCall(t, f, "GET", path, session(t, f, host), false)
		if r.status != 409 || r.body["error"].(map[string]any)["code"] != "TABLE_NOT_AVAILABLE" {
			t.Fatal(r.body)
		}
	}
}
