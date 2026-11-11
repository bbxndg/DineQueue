//go:build integration

package queue

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"dinequeue.local/api/internal/auth"
	"github.com/google/uuid"
)

func getTicket(t *testing.T, f fixture, path, token string) reply {
	t.Helper()
	req := httptest.NewRequest("GET", "/api/v1/me/queue-entries/"+path, nil)
	if token != "" {
		req.AddCookie(&http.Cookie{Name: f.auth.CookieName, Value: token})
	}
	return send(t, f, req)
}

func activeTickets(t *testing.T, r reply) []any {
	t.Helper()
	entries, ok := r.body["entries"].([]any)
	if r.status != 200 || !ok {
		t.Fatalf("expected 200 JSON entries array, got %d %v", r.status, r.body)
	}
	return entries
}

func TestF03CustomerRoleRequired(t *testing.T) {
	f := setup(t)
	hostToken := session(t, f, host)
	for _, path := range []string{"active", uuid.NewString(), "malformed"} {
		if r := getTicket(t, f, path, ""); r.status != 401 || errorCode(r) != "UNAUTHENTICATED" {
			t.Fatalf("T-021 guest %s: %d %v", path, r.status, r.body)
		}
		if r := getTicket(t, f, path, hostToken); r.status != 403 || errorCode(r) != "FORBIDDEN" {
			t.Fatalf("Host %s: %d %v", path, r.status, r.body)
		}
	}
	for _, role := range []string{"HOST", "", "MANAGER"} {
		user := auth.User{ID: customer1, Role: role}
		if _, err := f.service.ActiveEntries(context.Background(), user); err != auth.ErrForbidden {
			t.Errorf("service active accepted role %q: %v", role, err)
		}
		if _, err := f.service.EntryDetail(context.Background(), user, uuid.NewString()); err != auth.ErrForbidden {
			t.Errorf("service detail accepted role %q: %v", role, err)
		}
	}
}

func TestF03PersistedValuesLifecycleAndPrivacy(t *testing.T) {
	f := setup(t)
	token1, token2 := session(t, f, customer1), session(t, f, customer2)
	if entries := activeTickets(t, getTicket(t, f, "active", token1)); len(entries) != 0 {
		t.Fatal("T-020 empty customer received tickets")
	}
	// Prior business day: active reads must not silently filter by today's date.
	joined := time.Date(2026, 11, 5, 8, 15, 30, 123456000, time.UTC)
	f.service.now = func() time.Time { return joined }
	created := join(t, f, auth.DemoQueueID, `{"partySize":10}`, token1, true)
	if created.status != 201 {
		t.Fatalf("join: %d %v", created.status, created.body)
	}
	id := created.body["id"].(string)
	if other := join(t, f, auth.DemoQueueID, `{"partySize":2}`, token2, true); other.status != 201 {
		t.Fatalf("other join: %d", other.status)
	}
	var persisted struct {
		ServiceDate string
		JoinedAt    time.Time
		PartySize   int
	}
	if err := f.db.Raw("SELECT service_date::text, joined_at, party_size FROM queue_entries WHERE id = ?", id).Scan(&persisted).Error; err != nil {
		t.Fatal(err)
	}
	assertTicket := func(body map[string]any, status string) {
		t.Helper()
		if len(body) != 10 || body["id"] != id || body["queueId"] != auth.DemoQueueID || body["ticketCode"] != created.body["ticketCode"] || body["partySize"] != float64(persisted.PartySize) || body["status"] != status || body["serviceDate"] != persisted.ServiceDate {
			t.Fatalf("T-018 DTO/persisted mismatch: %v", body)
		}
		stamp, err := time.Parse(time.RFC3339Nano, body["joinedAt"].(string))
		if err != nil || !stamp.Equal(persisted.JoinedAt) || !stamp.Equal(joined) {
			t.Fatalf("T-018 timestamp mismatch: %v %v", body["joinedAt"], err)
		}
		if body["queueName"] != "Walk-in Dining" || body["restaurantName"] != "DineQueue Demo" || body["timezone"] != "Asia/Bangkok" {
			t.Fatalf("T-018 queue metadata: %v", body)
		}
		for _, key := range []string{"customerId", "customer_id", "email", "displayName", "passwordHash", "tokenHash", "sequenceNumber", "calledBy", "completedBy"} {
			if _, ok := body[key]; ok {
				t.Fatalf("private field %s exposed", key)
			}
		}
	}
	entries := activeTickets(t, getTicket(t, f, "active", token1))
	if len(entries) != 1 {
		t.Fatalf("T-018 expected only own ticket: %v", entries)
	}
	assertTicket(entries[0].(map[string]any), "WAITING")
	detail := getTicket(t, f, id, token1)
	if detail.status != 200 {
		t.Fatalf("T-018 detail %d", detail.status)
	}
	assertTicket(detail.body, "WAITING")

	// Test fixtures, not F-04 implementation: prove read behavior for stored states.
	update := f.db.Exec("UPDATE queue_entries SET status='CALLED', called_at=?, called_by=? WHERE id=?", joined.Add(time.Minute), host, id)
	if update.Error != nil || update.RowsAffected != 1 {
		t.Fatalf("called fixture: %v", update.Error)
	}
	if err := f.db.Exec("UPDATE service_queues SET is_active=false WHERE id=?", auth.DemoQueueID).Error; err != nil {
		t.Fatal(err)
	}
	entries = activeTickets(t, getTicket(t, f, "active", token1))
	if len(entries) != 1 {
		t.Fatal("inactive queue hid an active ticket")
	}
	assertTicket(entries[0].(map[string]any), "CALLED")
	update = f.db.Exec("UPDATE queue_entries SET status='COMPLETED', completed_at=?, completed_by=? WHERE id=?", joined.Add(2*time.Minute), host, id)
	if update.Error != nil || update.RowsAffected != 1 {
		t.Fatalf("completed fixture: %v", update.Error)
	}
	if entries := activeTickets(t, getTicket(t, f, "active", token1)); len(entries) != 0 {
		t.Fatal("COMPLETED is active")
	}
	detail = getTicket(t, f, id, token1)
	if detail.status != 200 {
		t.Fatal("owner lost completed detail")
	}
	assertTicket(detail.body, "COMPLETED")

	for _, path := range []string{id, uuid.NewString(), "not-a-uuid", "urn:uuid:" + id, "' OR 1=1 --"} {
		// Use a valid encoded path for the SQL-looking input.
		if path == "' OR 1=1 --" {
			path = "%27%20OR%201=1%20--"
		}
		r := getTicket(t, f, path, token2)
		if r.status != 404 || len(r.body) != 1 || errorCode(r) != "NOT_FOUND" {
			t.Fatalf("T-008 foreign/unknown detail: %d %v", r.status, r.body)
		}
		errorBody := r.body["error"].(map[string]any)
		if len(errorBody) != 3 || errorBody["message"] != "Resource not found." {
			t.Fatalf("revealing error: %v", r.body)
		}
	}
	if _, err := f.service.EntryDetail(context.Background(), auth.User{ID: customer2, Role: "CUSTOMER"}, id); err != ErrNotFound {
		t.Fatalf("service exposed foreign ticket: %v", err)
	}
	if err := f.db.Exec("UPDATE sessions SET revoked_at=now() WHERE token_hash=?", auth.TokenHash(token1)).Error; err != nil {
		t.Fatal(err)
	}
	if r := getTicket(t, f, id, token1); r.status != 401 {
		t.Fatal("revoked session read a ticket")
	}
}

func TestF03ActiveAcrossQueuesHasStableOrderAndNoCache(t *testing.T) {
	f := setup(t)
	token := session(t, f, customer1)
	joined := time.Now().UTC().Truncate(time.Microsecond)
	f.service.now = func() time.Time { return joined }
	first := join(t, f, auth.DemoQueueID, `{"partySize":3}`, token, true)
	queue2 := uuid.NewString()
	if err := f.db.Exec("INSERT INTO service_queues(id,restaurant_id,name,ticket_prefix,is_active) SELECT ?,restaurant_id,'Second queue','B',true FROM service_queues WHERE id=?", queue2, auth.DemoQueueID).Error; err != nil {
		t.Fatal(err)
	}
	second := join(t, f, queue2, `{"partySize":4}`, token, true)
	if first.status != 201 || second.status != 201 {
		t.Fatal("multi-queue setup failed")
	}
	queue3 := uuid.NewString()
	if err := f.db.Exec("INSERT INTO service_queues(id,restaurant_id,name,ticket_prefix,is_active) SELECT ?,restaurant_id,'Earlier queue','C',true FROM service_queues WHERE id=?", queue3, auth.DemoQueueID).Error; err != nil {
		t.Fatal(err)
	}
	f.service.now = func() time.Time { return joined.Add(-time.Minute) }
	earlier := join(t, f, queue3, `{"partySize":2}`, token, true)
	if earlier.status != 201 {
		t.Fatalf("earlier ticket setup failed: %d %v", earlier.status, earlier.body)
	}
	// The oldest ticket sorts last by ID, so ID-only ordering cannot satisfy this test.
	earliestID := "ffffffff-ffff-ffff-ffff-ffffffffffff"
	update := f.db.Exec("UPDATE queue_entries SET id=? WHERE id=?", earliestID, earlier.body["id"])
	if update.Error != nil || update.RowsAffected != 1 {
		t.Fatalf("earlier ID fixture: rows=%d error=%v", update.RowsAffected, update.Error)
	}
	entries := activeTickets(t, getTicket(t, f, "active", token))
	ids := []string{first.body["id"].(string), second.body["id"].(string)}
	sort.Strings(ids)
	expectedIDs := append([]string{earliestID}, ids...)
	if len(entries) != len(expectedIDs) {
		t.Fatal("missing own queue")
	}
	for i, entry := range entries {
		if entry.(map[string]any)["id"] != expectedIDs[i] {
			t.Fatal("unstable joined_at/id ordering")
		}
	}
	for _, path := range []string{"active", ids[0], uuid.NewString()} {
		req := httptest.NewRequest("GET", "/api/v1/me/queue-entries/"+path, nil)
		req.AddCookie(&http.Cookie{Name: f.auth.CookieName, Value: token})
		res, err := f.app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.Header.Get("Cache-Control") != "no-store" {
			t.Fatalf("private read cacheable: %v", res.Header)
		}
	}
}
