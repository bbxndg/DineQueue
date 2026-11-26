//go:build integration

package queue

import (
	"context"
	schema "dinequeue.local/api/db"
	"dinequeue.local/api/internal/auth"
	"errors"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func readinessPatch(t *testing.T, f fixture, id, token, body string, csrf bool) reply {
	t.Helper()
	req := httptest.NewRequest("PATCH", "/api/v1/me/queue-entries/"+id+"/readiness", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", testOrigin)
	if token != "" {
		req.AddCookie(&http.Cookie{Name: f.auth.CookieName, Value: token})
		if csrf {
			req.Header.Set("X-CSRF-Token", auth.CSRFToken(token))
		}
	}
	return send(t, f, req)
}
func ticketOwner(t *testing.T, f fixture, id string) string {
	t.Helper()
	var owner string
	if err := f.db.Raw("SELECT customer_id FROM queue_entries WHERE id=?", id).Scan(&owner).Error; err != nil {
		t.Fatal(err)
	}
	return owner
}
func TestL7ReadinessOwnerLifecycleAndFIFO(t *testing.T) {
	f := setup(t)
	a, b, c := joinNotReady(t, f), joinNotReady(t, f), joinNotReady(t, f)
	aid, bid, cid := a.body["id"].(string), b.body["id"].(string), c.body["id"].(string)
	atoken := session(t, f, ticketOwner(t, f, aid))
	hostToken := session(t, f, host)
	for _, id := range []string{bid, cid} {
		r := readinessPatch(t, f, id, session(t, f, ticketOwner(t, f, id)), `{"isReady":true}`, true)
		if r.status != 200 || r.body["isReady"] != true {
			t.Fatalf("ready %v", r)
		}
	}
	r := hostCall(t, f, "POST", hostPath(auth.DemoQueueID, "/call-next"), hostToken, true)
	if r.status != 200 || r.body["id"] != bid || r.body["isReady"] != true {
		t.Fatalf("must skip A and call B: %v", r)
	}
	before := entryState(t, f, aid)
	r = readinessPatch(t, f, aid, atoken, `{"isReady":true}`, true)
	if r.status != 200 || r.body["ticketCode"] != a.body["ticketCode"] || r.body["joinedAt"] != a.body["joinedAt"] {
		t.Fatalf("identity/order %v", r)
	}
	v := entryState(t, f, aid).Version
	if v != before.Version+1 {
		t.Fatal("readiness version not incremented")
	}
	if r = readinessPatch(t, f, aid, atoken, `{"isReady":true}`, true); r.status != 200 || entryState(t, f, aid).Version != v {
		t.Fatal("same-value set must be idempotent")
	}
	if r = hostCall(t, f, "POST", hostPath(auth.DemoQueueID, "/call-next"), hostToken, true); errorCode(r) != "CALL_IN_PROGRESS" {
		t.Fatal("must not displace B")
	}
	if r = hostCall(t, f, "POST", hostPath(auth.DemoQueueID, "/entries/"+bid+"/complete"), hostToken, true); r.status != 200 {
		t.Fatal(r)
	}
	r = hostCall(t, f, "POST", hostPath(auth.DemoQueueID, "/call-next"), hostToken, true)
	if r.status != 200 || r.body["id"] != aid {
		t.Fatalf("A must return ahead of C: %v", r)
	}
	r = readinessPatch(t, f, aid, atoken, `{"isReady":false}`, true)
	if r.status != 409 || errorCode(r) != "INVALID_TRANSITION" {
		t.Fatalf("called guard %v", r)
	}
	hostCall(t, f, "POST", hostPath(auth.DemoQueueID, "/entries/"+aid+"/complete"), hostToken, true)
	if r = readinessPatch(t, f, aid, atoken, `{"isReady":false}`, true); r.status != 409 {
		t.Fatal("completed guard", r)
	}
	noPrivateFields(t, "readiness", r.body)
}
func TestL7ReadinessValidationPrivacyAndCSRF(t *testing.T) {
	f := setup(t)
	a := joinNotReady(t, f)
	id := a.body["id"].(string)
	owner := session(t, f, ticketOwner(t, f, id))
	for _, tc := range []struct {
		label, id, token, body string
		csrf                   bool
		want                   int
	}{
		{"guest", id, "", `{"isReady":true}`, false, 401},
		{"host", id, session(t, f, host), `{"isReady":true}`, true, 403},
		{"foreign", id, session(t, f, customer1), `{"isReady":true}`, true, 404},
		{"missing", uuid.NewString(), owner, `{"isReady":true}`, true, 404},
		{"malformed", "invalid", owner, `{"isReady":true}`, true, 404},
		{"csrf", id, owner, `{"isReady":true}`, false, 403},
		{"null", id, owner, `{"isReady":null}`, true, 400},
		{"number", id, owner, `{"isReady":1}`, true, 400},
		{"unknown", id, owner, `{"isReady":true,"customerId":"bad"}`, true, 400},
		{"duplicate", id, owner, `{"isReady":true,"isReady":false}`, true, 400},
		{"empty", id, owner, `{}`, true, 400},
	} {
		t.Run(tc.label, func(t *testing.T) {
			before, err := f.service.EntryDetail(context.Background(), auth.User{ID: ticketOwner(t, f, id), Role: "CUSTOMER"}, id)
			if err != nil {
				t.Fatal(err)
			}
			beforeVersion := entryState(t, f, id).Version
			r := readinessPatch(t, f, tc.id, tc.token, tc.body, tc.csrf)
			if r.status != tc.want {
				t.Fatalf("%s %v", tc.label, r)
			}
			noPrivateFields(t, tc.label, r.body)
			after, err := f.service.EntryDetail(context.Background(), auth.User{ID: ticketOwner(t, f, id), Role: "CUSTOMER"}, id)
			if err != nil {
				t.Fatal(err)
			}
			if after.IsReady != before.IsReady || after.Status != before.Status || entryState(t, f, id).Version != beforeVersion {
				t.Fatalf("rejected %s request changed persisted readiness/status/version", tc.label)
			}
		})
	}
	r := readinessPatch(t, f, id, owner, `{"isReady":true}`, true)
	if r.status != 200 {
		t.Fatal(r)
	}
	noPrivateFields(t, "success", r.body)
	list := hostCall(t, f, "GET", hostPath(auth.DemoQueueID, "/entries?status=WAITING"), session(t, f, host), false)
	if list.status != 200 || list.body["entries"].([]any)[0].(map[string]any)["isReady"] != true {
		t.Fatal("host projection", list)
	}
}
func TestL7ReadinessInactiveQueueAndPersistence(t *testing.T) {
	f := setup(t)
	a := joinNotReady(t, f)
	id := a.body["id"].(string)
	uid := ticketOwner(t, f, id)
	if err := f.db.Exec("UPDATE service_queues SET is_active=false WHERE id=?", auth.DemoQueueID).Error; err != nil {
		t.Fatal(err)
	}
	if r := readinessPatch(t, f, id, session(t, f, uid), `{"isReady":true}`, true); r.status != 200 {
		t.Fatal(r)
	}
	// New service instance reads DB state; full process restart is covered separately in browser smoke.
	fresh := NewService(NewRepository(f.db))
	got, err := fresh.EntryDetail(context.Background(), auth.User{ID: uid, Role: "CUSTOMER"}, id)
	if err != nil || !got.IsReady {
		t.Fatal("persisted readiness", got, err)
	}
	if r := join(t, f, auth.DemoQueueID, `{"partySize":2}`, session(t, f, newUser(t, f, "CUSTOMER")), true); r.status != 409 || errorCode(r) != "QUEUE_INACTIVE" {
		t.Fatal(r)
	}
	if _, err := fresh.CallNext(context.Background(), auth.User{ID: host, Role: "HOST"}, auth.DemoQueueID); err != nil {
		t.Fatal(err)
	}
}
func TestL7ReadinessMigrationPreservesExistingTickets(t *testing.T) {
	f := setup(t)
	a := joinNotReady(t, f)
	id := a.body["id"].(string)
	if err := f.db.Exec("ALTER TABLE queue_entries DROP COLUMN is_ready").Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Exec("DELETE FROM schema_migrations WHERE version='0003_customer_readiness.sql'").Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := schema.Migrate(context.Background(), f.db); err != nil {
			t.Fatal(err)
		}
	}
	row, err := f.service.EntryDetail(context.Background(), auth.User{ID: ticketOwner(t, f, id), Role: "CUSTOMER"}, id)
	if err != nil || !row.IsReady || row.TicketCode != a.body["ticketCode"] || row.JoinedAt.Format(time.RFC3339Nano) != a.body["joinedAt"] {
		t.Fatal("migration lost identity", row, err)
	}
	if r := joinNotReady(t, f); r.body["isReady"] != false {
		t.Fatal("new default", r)
	}
}

// Wait for PostgreSQL to confirm an actual blocked lock request. This makes the
// ordering tests deterministic rather than hoping goroutines run in a given order.
func awaitQueueWaiter(t *testing.T, f fixture, blocker int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var n int64
		if err := f.db.Raw("SELECT count(*) FROM pg_stat_activity WHERE ? = ANY(pg_blocking_pids(pid))", blocker).Scan(&n).Error; err != nil {
			t.Fatal(err)
		}
		if n > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("operation never reached the queue lock")
}
func TestL7ReadinessCallRaceBothCommitOrders(t *testing.T) {
	for _, readinessFirst := range []bool{true, false} {
		t.Run(map[bool]string{true: "not-ready-first", false: "call-first"}[readinessFirst], func(t *testing.T) {
			f := setup(t)
			a := joinAs(t, f)
			id := a.body["id"].(string)
			owner := auth.User{ID: ticketOwner(t, f, id), Role: "CUSTOMER"}
			h := auth.User{ID: host, Role: "HOST"}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			tx := f.db.WithContext(ctx).Begin()
			if tx.Error != nil {
				t.Fatal(tx.Error)
			}
			defer tx.Rollback()
			var pid int
			if err := tx.Raw("SELECT pg_backend_pid()").Scan(&pid).Error; err != nil {
				t.Fatal(err)
			}
			if _, err := NewRepository(tx).lockQueue(ctx, auth.DemoQueueID); err != nil {
				t.Fatal(err)
			}
			result := make(chan error, 1)
			if readinessFirst {
				go func() { _, err := f.service.CallNext(ctx, h, auth.DemoQueueID); result <- err }()
			} else {
				go func() { _, err := f.service.SetReadiness(ctx, owner, id, false); result <- err }()
			}
			awaitQueueWaiter(t, f, pid)
			service := NewService(NewRepository(tx))
			if readinessFirst {
				if _, err := service.SetReadiness(ctx, owner, id, false); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := service.CallNext(ctx, h, auth.DemoQueueID); err != nil {
					t.Fatal(err)
				}
			}
			if err := tx.Commit().Error; err != nil {
				t.Fatal(err)
			}
			want := ErrInvalidTransition
			if readinessFirst {
				want = ErrNoReadyParties
			}
			select {
			case err := <-result:
				if !errors.Is(err, want) {
					t.Fatalf("got %v want %v", err, want)
				}
			case <-ctx.Done():
				t.Fatal("race timed out")
			}
			row, err := f.service.EntryDetail(ctx, owner, id)
			if err != nil {
				t.Fatal(err)
			}
			if readinessFirst {
				if row.IsReady || row.Status != "WAITING" {
					t.Fatal(row)
				}
			} else {
				if !row.IsReady || row.Status != "CALLED" {
					t.Fatal(row)
				}
			}
		})
	}
}
