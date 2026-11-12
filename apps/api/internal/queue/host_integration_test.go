//go:build integration

package queue

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"dinequeue.local/api/internal/auth"
	"github.com/google/uuid"
)

const otherHost = "20000000-0000-4000-8000-000000000004" // seeded, no grant on the demo queue

func hostCall(t *testing.T, f fixture, method, path, token string, csrf bool) reply {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Origin", testOrigin)
	if token != "" {
		req.AddCookie(&http.Cookie{Name: f.auth.CookieName, Value: token})
		if csrf {
			req.Header.Set("X-CSRF-Token", auth.CSRFToken(token))
		}
	}
	return send(t, f, req)
}

func hostPath(queueID, suffix string) string { return "/api/v1/host/queues/" + queueID + suffix }

func newUser(t *testing.T, f fixture, role string) string {
	t.Helper()
	id := uuid.NewString()
	if err := f.db.Exec("INSERT INTO users(id,display_name,email,password_hash,role) VALUES (?,?,?,?,?)",
		id, "Fixture "+role, id+"@example.test", "not-a-login-hash", role).Error; err != nil {
		t.Fatal(err)
	}
	return id
}

// grantedHost creates a second Host (H3) with a grant on the queue.
func grantedHost(t *testing.T, f fixture, queueID string) string {
	t.Helper()
	id := newUser(t, f, "HOST")
	if err := f.db.Exec("INSERT INTO operator_queue_permissions(user_id,queue_id) VALUES (?,?)", id, queueID).Error; err != nil {
		t.Fatal(err)
	}
	return id
}

// waitingEntry inserts a WAITING fixture with a fixed joined_at for deterministic FIFO.
func waitingEntry(t *testing.T, f fixture, queueID string, sequence int, joinedAt time.Time) string {
	t.Helper()
	id := uuid.NewString()
	if err := f.db.Exec(`INSERT INTO queue_entries(id,queue_id,customer_id,service_date,sequence_number,party_size,status,joined_at)
VALUES (?,?,?,?,?,2,'WAITING',?)`, id, queueID, newUser(t, f, "CUSTOMER"), joinedAt.Format(time.DateOnly), sequence, joinedAt).Error; err != nil {
		t.Fatal(err)
	}
	return id
}

type lifecycle struct {
	Status                string
	CalledBy, CompletedBy *string
	CalledAt, CompletedAt *time.Time
	Version               int
}

func entryState(t *testing.T, f fixture, id string) lifecycle {
	t.Helper()
	var row lifecycle
	if err := f.db.Table("queue_entries").Select("status,called_by,completed_by,called_at,completed_at,version").Where("id = ?", id).Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	return row
}

func statusCount(t *testing.T, f fixture, status string) int64 {
	var n int64
	f.db.Table("queue_entries").Where("status = ?", status).Count(&n)
	return n
}

func joinAs(t *testing.T, f fixture) reply {
	t.Helper()
	r := join(t, f, auth.DemoQueueID, `{"partySize":2}`, session(t, f, newUser(t, f, "CUSTOMER")), true)
	if r.status != 201 {
		t.Fatalf("fixture join %d %v", r.status, r.body)
	}
	return r
}

func noPrivateFields(t *testing.T, label string, body map[string]any) {
	t.Helper()
	for _, private := range []string{"customerId", "email", "displayName", "sequenceNumber", "calledBy", "completedBy"} {
		if _, ok := body[private]; ok {
			t.Errorf("%s exposes %s", label, private)
		}
	}
}

func TestF04WaitingListIsFIFO(t *testing.T) {
	f := setup(t)
	base := time.Date(2026, 11, 11, 3, 0, 0, 0, time.UTC)
	third := waitingEntry(t, f, auth.DemoQueueID, 3, base.Add(2*time.Minute))
	first := waitingEntry(t, f, auth.DemoQueueID, 1, base)
	second := waitingEntry(t, f, auth.DemoQueueID, 2, base.Add(time.Minute))
	token := session(t, f, host)
	r := hostCall(t, f, "GET", hostPath(auth.DemoQueueID, "/entries?status=WAITING"), token, false)
	entries, _ := r.body["entries"].([]any)
	if r.status != 200 || len(entries) != 3 {
		t.Fatalf("T-022 %d %v", r.status, r.body)
	}
	for i, want := range []string{first, second, third} {
		row := entries[i].(map[string]any)
		if row["id"] != want || row["status"] != "WAITING" {
			t.Fatalf("T-022 position %d got %v", i, row)
		}
		noPrivateFields(t, "T-022", row)
	}
	if entries[0].(map[string]any)["ticketCode"] != "A-001" {
		t.Fatalf("T-022 ticket code %v", entries[0])
	}
	// Status defaults to WAITING; any other filter is rejected.
	if r := hostCall(t, f, "GET", hostPath(auth.DemoQueueID, "/entries"), token, false); r.status != 200 || len(r.body["entries"].([]any)) != 3 {
		t.Fatalf("default filter %d %v", r.status, r.body)
	}
	if r := hostCall(t, f, "GET", hostPath(auth.DemoQueueID, "/entries?status=CALLED"), token, false); r.status != 400 || errorCode(r) != "VALIDATION_ERROR" {
		t.Fatalf("bad filter %d %v", r.status, r.body)
	}
	if r := hostCall(t, f, "GET", hostPath(auth.DemoQueueID, "/current"), token, false); r.status != 200 || r.body["entry"] != nil {
		t.Fatalf("empty current %d %v", r.status, r.body)
	}
	// Assigned queue list contains only granted queues.
	if r := hostCall(t, f, "GET", "/api/v1/host/queues", token, false); r.status != 200 || len(r.body["queues"].([]any)) != 1 || r.body["queues"].([]any)[0].(map[string]any)["id"] != auth.DemoQueueID {
		t.Fatalf("host queues %d %v", r.status, r.body)
	}
	if r := hostCall(t, f, "GET", "/api/v1/host/queues", session(t, f, otherHost), false); r.status != 200 || len(r.body["queues"].([]any)) != 0 {
		t.Fatalf("unassigned host queues %d %v", r.status, r.body)
	}
}

func TestF04CallNextCallsOldestWaiting(t *testing.T) {
	f := setup(t)
	a1 := joinAs(t, f)
	a2 := joinAs(t, f)
	token := session(t, f, host)
	r := hostCall(t, f, "POST", hostPath(auth.DemoQueueID, "/call-next"), token, true)
	if r.status != 200 || r.body["id"] != a1.body["id"] || r.body["ticketCode"] != "A-001" || r.body["status"] != "CALLED" || r.body["calledAt"] == nil {
		t.Fatalf("T-023 %d %v", r.status, r.body)
	}
	noPrivateFields(t, "T-023", r.body)
	state := entryState(t, f, a1.body["id"].(string))
	if state.Status != "CALLED" || state.CalledBy == nil || *state.CalledBy != host || state.CalledAt == nil || state.Version != 2 {
		t.Fatalf("T-023 persisted %+v", state)
	}
	if entryState(t, f, a2.body["id"].(string)).Status != "WAITING" {
		t.Fatal("T-023 A-002 changed")
	}
	current := hostCall(t, f, "GET", hostPath(auth.DemoQueueID, "/current"), token, false)
	entry, _ := current.body["entry"].(map[string]any)
	if current.status != 200 || entry["id"] != a1.body["id"] {
		t.Fatalf("current %d %v", current.status, current.body)
	}
	list := hostCall(t, f, "GET", hostPath(auth.DemoQueueID, "/entries?status=WAITING"), token, false)
	if rows := list.body["entries"].([]any); len(rows) != 1 || rows[0].(map[string]any)["id"] != a2.body["id"] {
		t.Fatalf("waiting after call %v", list.body)
	}
}

func TestF04CallNextBusinessConflicts(t *testing.T) {
	f := setup(t)
	token := session(t, f, host)
	if r := hostCall(t, f, "POST", hostPath(auth.DemoQueueID, "/call-next"), token, true); r.status != 409 || errorCode(r) != "QUEUE_EMPTY" {
		t.Fatalf("T-024 %d %v", r.status, r.body)
	}
	if count(t, f, "queue_entries") != 0 {
		t.Fatal("T-024 wrote data")
	}
	a1 := joinAs(t, f)
	a2 := joinAs(t, f)
	if r := hostCall(t, f, "POST", hostPath(auth.DemoQueueID, "/call-next"), token, true); r.status != 200 {
		t.Fatalf("call %d %v", r.status, r.body)
	}
	before := entryState(t, f, a1.body["id"].(string))
	if r := hostCall(t, f, "POST", hostPath(auth.DemoQueueID, "/call-next"), token, true); r.status != 409 || errorCode(r) != "CALL_IN_PROGRESS" {
		t.Fatalf("T-025 %d %v", r.status, r.body)
	}
	after := entryState(t, f, a1.body["id"].(string))
	if after.Version != before.Version || after.Status != "CALLED" || entryState(t, f, a2.body["id"].(string)).Status != "WAITING" {
		t.Fatal("T-025 entries changed")
	}
}

func TestF04ConcurrentCallNextClaimsOnce(t *testing.T) {
	f := setup(t)
	for i := 0; i < 3; i++ {
		joinAs(t, f)
	}
	tokens := []string{session(t, f, host), session(t, f, grantedHost(t, f, auth.DemoQueueID))}
	results := make([]reply, 10)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results[i] = hostCall(t, f, "POST", hostPath(auth.DemoQueueID, "/call-next"), tokens[i%2], true)
		}(i)
	}
	close(start)
	wg.Wait()
	called := 0
	for _, r := range results {
		switch {
		case r.status == 200:
			called++
		case r.status == 409 && errorCode(r) == "CALL_IN_PROGRESS":
		default:
			t.Fatalf("T-026 unexpected %d %v", r.status, r.body)
		}
	}
	if called != 1 || statusCount(t, f, "CALLED") != 1 || statusCount(t, f, "WAITING") != 2 {
		t.Fatalf("T-026 called=%d rows CALLED=%d WAITING=%d", called, statusCount(t, f, "CALLED"), statusCount(t, f, "WAITING"))
	}
	var code string
	f.db.Raw("SELECT sequence_number::text FROM queue_entries WHERE status='CALLED'").Scan(&code)
	if code != "1" {
		t.Fatalf("T-026 called sequence %s, want oldest", code)
	}
}

func TestF04AccessControl(t *testing.T) {
	f := setup(t)
	a1 := joinAs(t, f)
	entry := a1.body["id"].(string)
	customer := session(t, f, customer1)
	unassigned := session(t, f, otherHost)
	paths := []struct{ method, path string }{
		{"GET", hostPath(auth.DemoQueueID, "/entries?status=WAITING")},
		{"GET", hostPath(auth.DemoQueueID, "/current")},
		{"POST", hostPath(auth.DemoQueueID, "/call-next")},
		{"POST", hostPath(auth.DemoQueueID, "/entries/"+entry+"/complete")},
	}
	for _, p := range paths {
		if r := hostCall(t, f, p.method, p.path, "", false); r.status != 401 {
			t.Errorf("T-005 guest %s %s: %d", p.method, p.path, r.status)
		}
		if r := hostCall(t, f, p.method, p.path, customer, true); r.status != 403 || errorCode(r) != "FORBIDDEN" {
			t.Errorf("T-006/T-032 customer %s %s: %d %s", p.method, p.path, r.status, errorCode(r))
		}
		if r := hostCall(t, f, p.method, p.path, unassigned, true); r.status != 403 || errorCode(r) != "FORBIDDEN" {
			t.Errorf("T-007/T-027 unassigned %s %s: %d %s", p.method, p.path, r.status, errorCode(r))
		}
	}
	if r := hostCall(t, f, "GET", "/api/v1/host/queues", customer, false); r.status != 403 {
		t.Errorf("customer host queue list %d", r.status)
	}
	assigned := session(t, f, host)
	// Unknown well-formed queue is not granted (403); malformed IDs are 404.
	if r := hostCall(t, f, "POST", hostPath(uuid.NewString(), "/call-next"), assigned, true); r.status != 403 {
		t.Errorf("unknown queue %d", r.status)
	}
	if r := hostCall(t, f, "POST", hostPath("not-a-uuid", "/call-next"), assigned, true); r.status != 404 {
		t.Errorf("malformed queue %d", r.status)
	}
	// Mutations require CSRF + exact Origin.
	if r := hostCall(t, f, "POST", hostPath(auth.DemoQueueID, "/call-next"), assigned, false); r.status != 403 || errorCode(r) != "CSRF_REJECTED" {
		t.Errorf("call-next without CSRF %d %s", r.status, errorCode(r))
	}
	req := httptest.NewRequest("POST", hostPath(auth.DemoQueueID, "/call-next"), nil)
	req.Header.Set("Origin", "https://evil.example.test")
	req.AddCookie(&http.Cookie{Name: f.auth.CookieName, Value: assigned})
	req.Header.Set("X-CSRF-Token", auth.CSRFToken(assigned))
	if r := send(t, f, req); r.status != 403 || errorCode(r) != "CSRF_REJECTED" {
		t.Errorf("call-next foreign origin %d %s", r.status, errorCode(r))
	}
	// Non-empty body on call-next is rejected.
	req = httptest.NewRequest("POST", hostPath(auth.DemoQueueID, "/call-next"), strings.NewReader(`{"entryId":"x"}`))
	req.Header.Set("Origin", testOrigin)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: f.auth.CookieName, Value: assigned})
	req.Header.Set("X-CSRF-Token", auth.CSRFToken(assigned))
	if r := send(t, f, req); r.status != 400 {
		t.Errorf("call-next body %d", r.status)
	}
	// Service rechecks role and grant independently of the HTTP gates.
	ctx := context.Background()
	if _, err := f.service.CallNext(ctx, auth.User{ID: customer1, Role: "CUSTOMER"}, auth.DemoQueueID); !errors.Is(err, auth.ErrForbidden) {
		t.Errorf("service customer call-next %v", err)
	}
	if _, err := f.service.CallNext(ctx, auth.User{ID: otherHost, Role: "HOST"}, auth.DemoQueueID); !errors.Is(err, auth.ErrForbidden) {
		t.Errorf("service unassigned call-next %v", err)
	}
	if _, err := f.service.Complete(ctx, auth.User{ID: otherHost, Role: "HOST"}, auth.DemoQueueID, entry); !errors.Is(err, auth.ErrForbidden) {
		t.Errorf("service unassigned complete %v", err)
	}
	if _, err := f.service.WaitingEntries(ctx, auth.User{ID: otherHost, Role: "HOST"}, auth.DemoQueueID); !errors.Is(err, auth.ErrForbidden) {
		t.Errorf("service unassigned list %v", err)
	}
	if state := entryState(t, f, entry); state.Status != "WAITING" || state.Version != 1 {
		t.Fatalf("rejected requests changed entry %+v", state)
	}
	// A grant that is revoked takes effect on the next request.
	if result := f.db.Exec("DELETE FROM operator_queue_permissions WHERE user_id = ?", host); result.Error != nil || result.RowsAffected != 1 {
		t.Fatalf("revoke grant fixture: %v rows=%d", result.Error, result.RowsAffected)
	}
	if r := hostCall(t, f, "POST", hostPath(auth.DemoQueueID, "/call-next"), assigned, true); r.status != 403 {
		t.Errorf("revoked grant %d", r.status)
	}
}

func TestF04CompleteThenCallNext(t *testing.T) {
	f := setup(t)
	a1 := joinAs(t, f)
	a2 := joinAs(t, f)
	h3 := grantedHost(t, f, auth.DemoQueueID)
	token := session(t, f, host)
	if r := hostCall(t, f, "POST", hostPath(auth.DemoQueueID, "/call-next"), token, true); r.status != 200 {
		t.Fatalf("call %d %v", r.status, r.body)
	}
	// A different authorized Host completes; completed_by records that actor.
	id := a1.body["id"].(string)
	r := hostCall(t, f, "POST", hostPath(auth.DemoQueueID, "/entries/"+id+"/complete"), session(t, f, h3), true)
	if r.status != 200 || r.body["status"] != "COMPLETED" || r.body["completedAt"] == nil || r.body["id"] != id {
		t.Fatalf("T-028 %d %v", r.status, r.body)
	}
	noPrivateFields(t, "T-028", r.body)
	state := entryState(t, f, id)
	if state.Status != "COMPLETED" || state.CompletedBy == nil || *state.CompletedBy != h3 || *state.CalledBy != host ||
		state.CompletedAt == nil || state.CompletedAt.Before(*state.CalledAt) || state.Version != 3 {
		t.Fatalf("T-028 persisted %+v", state)
	}
	// Owner sees the completed ticket through the F-03 read.
	detail := getTicket(t, f, id, session(t, f, customerOf(t, f, id)))
	if detail.status != 200 || detail.body["status"] != "COMPLETED" {
		t.Fatalf("owner read %d %v", detail.status, detail.body)
	}
	if current := hostCall(t, f, "GET", hostPath(auth.DemoQueueID, "/current"), token, false); current.body["entry"] != nil {
		t.Fatalf("current after complete %v", current.body)
	}
	next := hostCall(t, f, "POST", hostPath(auth.DemoQueueID, "/call-next"), token, true)
	if next.status != 200 || next.body["id"] != a2.body["id"] || next.body["ticketCode"] != "A-002" {
		t.Fatalf("T-033 %d %v", next.status, next.body)
	}
}

func customerOf(t *testing.T, f fixture, entryID string) string {
	var id string
	f.db.Raw("SELECT customer_id::text FROM queue_entries WHERE id = ?", entryID).Scan(&id)
	return id
}

func TestF04InvalidCompletion(t *testing.T) {
	f := setup(t)
	a1 := joinAs(t, f)
	a2 := joinAs(t, f)
	token := session(t, f, host)
	complete := func(entryID string) reply {
		return hostCall(t, f, "POST", hostPath(auth.DemoQueueID, "/entries/"+entryID+"/complete"), token, true)
	}
	waiting := a1.body["id"].(string)
	if r := complete(waiting); r.status != 409 || errorCode(r) != "INVALID_TRANSITION" {
		t.Fatalf("T-030 %d %v", r.status, r.body)
	}
	if s := entryState(t, f, waiting); s.Status != "WAITING" || s.Version != 1 {
		t.Fatalf("T-030 changed %+v", s)
	}
	hostCall(t, f, "POST", hostPath(auth.DemoQueueID, "/call-next"), token, true)
	if r := complete(waiting); r.status != 200 {
		t.Fatalf("complete %d %v", r.status, r.body)
	}
	before := entryState(t, f, waiting)
	if r := complete(waiting); r.status != 409 || errorCode(r) != "INVALID_TRANSITION" {
		t.Fatalf("T-031 %d %v", r.status, r.body)
	}
	if after := entryState(t, f, waiting); after.Version != before.Version || !after.CompletedAt.Equal(*before.CompletedAt) {
		t.Fatal("T-031 completed entry changed")
	}
	for _, id := range []string{uuid.NewString(), "not-a-uuid"} {
		if r := complete(id); r.status != 404 || errorCode(r) != "NOT_FOUND" {
			t.Errorf("unknown entry %s: %d", id, r.status)
		}
	}
	// An entry from another queue is not reachable through this queue's path.
	otherQueue := uuid.NewString()
	if err := f.db.Exec("INSERT INTO service_queues(id,restaurant_id,name,ticket_prefix) VALUES (?,?,?,?)", otherQueue, "30000000-0000-4000-8000-000000000001", "Patio", "B").Error; err != nil {
		t.Fatalf("second queue fixture: %v", err)
	}
	if err := f.db.Exec("INSERT INTO operator_queue_permissions(user_id,queue_id) VALUES (?,?)", host, otherQueue).Error; err != nil {
		t.Fatalf("second queue grant fixture: %v", err)
	}
	foreign := waitingEntry(t, f, otherQueue, 1, time.Now().UTC().Add(-time.Minute))
	if r := hostCall(t, f, "POST", hostPath(otherQueue, "/call-next"), token, true); r.status != 200 || r.body["id"] != foreign {
		t.Fatalf("other queue call %d %v", r.status, r.body)
	}
	if r := complete(foreign); r.status != 404 {
		t.Fatalf("cross-queue complete %d", r.status)
	}
	if entryState(t, f, foreign).Status != "CALLED" || entryState(t, f, a2.body["id"].(string)).Status != "WAITING" {
		t.Fatal("cross-queue complete changed data")
	}
}

// A malformed entry ID is rejected before the transaction, so it never waits on
// (or holds) the queue row lock that Call Next and Complete share.
func TestF04MalformedEntryDoesNotTakeQueueLock(t *testing.T) {
	f := setup(t)
	holder := f.db.Begin()
	defer holder.Rollback()
	if err := holder.Exec("SELECT id FROM service_queues WHERE id = ? FOR UPDATE", auth.DemoQueueID).Error; err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := f.service.Complete(ctx, auth.User{ID: host, Role: "HOST"}, auth.DemoQueueID, "not-a-uuid"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("malformed entry while queue is locked: %v, want ErrNotFound without waiting", err)
	}
}

func TestF04ConcurrentCompleteSucceedsOnce(t *testing.T) {
	f := setup(t)
	a1 := joinAs(t, f)
	token := session(t, f, host)
	hostCall(t, f, "POST", hostPath(auth.DemoQueueID, "/call-next"), token, true)
	path := hostPath(auth.DemoQueueID, "/entries/"+a1.body["id"].(string)+"/complete")
	results := make([]reply, 6)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results[i] = hostCall(t, f, "POST", path, token, true)
		}(i)
	}
	close(start)
	wg.Wait()
	ok := 0
	for _, r := range results {
		if r.status == 200 {
			ok++
		} else if r.status != 409 || errorCode(r) != "INVALID_TRANSITION" {
			t.Fatalf("unexpected %d %v", r.status, r.body)
		}
	}
	if ok != 1 || entryState(t, f, a1.body["id"].(string)).Version != 3 {
		t.Fatalf("parallel complete ok=%d", ok)
	}
}
