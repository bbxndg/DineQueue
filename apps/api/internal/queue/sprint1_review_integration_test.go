//go:build integration

package queue

import (
	"strings"
	"sync"
	"testing"
	"time"

	"dinequeue.local/api/internal/auth"
)

func TestS1NoncanonicalQueueIDsReturnNotFound(t *testing.T) {
	f := setup(t)
	customerToken, hostToken := session(t, f, customer1), session(t, f, host)
	for _, id := range []string{"urn:uuid:" + auth.DemoQueueID, strings.ReplaceAll(auth.DemoQueueID, "-", ""), "{" + auth.DemoQueueID + "}"} {
		if r := join(t, f, id, `{"partySize":2}`, customerToken, true); r.status != 404 || errorCode(r) != "NOT_FOUND" {
			t.Errorf("join noncanonical queue: %d %v", r.status, r.body)
		}
		for _, route := range []struct{ method, suffix string }{
			{"GET", "/entries"}, {"GET", "/current"}, {"POST", "/call-next"},
			{"POST", "/entries/00000000-0000-4000-8000-000000000001/complete"},
		} {
			r := hostCall(t, f, route.method, hostPath(id, route.suffix), hostToken, true)
			if r.status != 404 || errorCode(r) != "NOT_FOUND" {
				t.Errorf("Host %s %s: %d %v", route.method, route.suffix, r.status, r.body)
			}
		}
	}
	if count(t, f, "queue_entries") != 0 || count(t, f, "queue_day_counters") != 0 {
		t.Fatal("rejected IDs changed ticket/counter data")
	}
}

func TestS1HostDrainsInactiveQueueInFIFOIncludingTiesAndPreviousDays(t *testing.T) {
	f := setup(t)
	base := time.Date(2026, 11, 10, 3, 0, 0, 0, time.UTC)
	high := waitingEntry(t, f, auth.DemoQueueID, 1, base)
	low := waitingEntry(t, f, auth.DemoQueueID, 2, base)
	later := waitingEntry(t, f, auth.DemoQueueID, 1, base.Add(24*time.Hour))
	for _, change := range []struct{ old, id string }{
		{high, "ffffffff-ffff-4fff-8fff-ffffffffffff"}, {low, "00000000-0000-4000-8000-000000000001"},
	} {
		if r := f.db.Exec("UPDATE queue_entries SET id=? WHERE id=?", change.id, change.old); r.Error != nil || r.RowsAffected != 1 {
			t.Fatalf("FIFO fixture: %v rows=%d", r.Error, r.RowsAffected)
		}
	}
	if r := f.db.Exec("UPDATE service_queues SET is_active=false WHERE id=?", auth.DemoQueueID); r.Error != nil || r.RowsAffected != 1 {
		t.Fatalf("close queue fixture: %v", r.Error)
	}
	want := []string{"00000000-0000-4000-8000-000000000001", "ffffffff-ffff-4fff-8fff-ffffffffffff", later}
	token := session(t, f, host)
	board := hostCall(t, f, "GET", hostPath(auth.DemoQueueID, "/entries"), token, false)
	if board.status != 200 {
		t.Fatalf("closed queue board: %d", board.status)
	}
	rows := board.body["entries"].([]any)
	if len(rows) != len(want) {
		t.Fatalf("FIFO list length %d", len(rows))
	}
	for i, id := range want {
		if rows[i].(map[string]any)["id"] != id {
			t.Fatalf("FIFO list position %d", i)
		}
		called := hostCall(t, f, "POST", hostPath(auth.DemoQueueID, "/call-next"), token, true)
		if called.status != 200 || called.body["id"] != id {
			t.Fatalf("FIFO call %d: %d %v", i, called.status, called.body)
		}
		completed := hostCall(t, f, "POST", hostPath(auth.DemoQueueID, "/entries/"+id+"/complete"), token, true)
		if completed.status != 200 {
			t.Fatalf("closed queue complete: %d %v", completed.status, completed.body)
		}
	}
	if statusCount(t, f, "COMPLETED") != 3 {
		t.Fatal("closed queue did not drain")
	}
}

func TestS1ConcurrentCompleteAndCallNextPreserveLifecycle(t *testing.T) {
	f := setup(t)
	a, b := joinAs(t, f), joinAs(t, f)
	first, second := a.body["id"].(string), b.body["id"].(string)
	token1, token2 := session(t, f, host), session(t, f, grantedHost(t, f, auth.DemoQueueID))
	if r := hostCall(t, f, "POST", hostPath(auth.DemoQueueID, "/call-next"), token1, true); r.status != 200 {
		t.Fatal("initial call failed")
	}
	var completed, called reply
	var wg sync.WaitGroup
	start := make(chan struct{})
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		completed = hostCall(t, f, "POST", hostPath(auth.DemoQueueID, "/entries/"+first+"/complete"), token1, true)
	}()
	go func() {
		defer wg.Done()
		<-start
		called = hostCall(t, f, "POST", hostPath(auth.DemoQueueID, "/call-next"), token2, true)
	}()
	close(start)
	wg.Wait()
	if completed.status != 200 {
		t.Fatalf("parallel complete: %d %v", completed.status, completed.body)
	}
	if called.status == 409 && errorCode(called) == "CALL_IN_PROGRESS" {
		called = hostCall(t, f, "POST", hostPath(auth.DemoQueueID, "/call-next"), token2, true)
	}
	if called.status != 200 || called.body["id"] != second || statusCount(t, f, "CALLED") != 1 || entryState(t, f, first).Status != "COMPLETED" {
		t.Fatalf("parallel lifecycle: call=%d %v", called.status, called.body)
	}
}

func TestS1HostTransitionSQLFailureRollsBack(t *testing.T) {
	f := setup(t)
	id := joinAs(t, f).body["id"].(string)
	token := session(t, f, host)
	if err := f.db.Exec(`CREATE FUNCTION reject_review_transition() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected transition failure'; END $$`).Error; err != nil {
		t.Fatal(err)
	}
	reject := func() {
		t.Helper()
		if err := f.db.Exec(`CREATE TRIGGER reject_review_transition BEFORE UPDATE ON queue_entries FOR EACH ROW EXECUTE FUNCTION reject_review_transition()`).Error; err != nil {
			t.Fatal(err)
		}
	}
	restore := func() {
		t.Helper()
		if err := f.db.Exec("DROP TRIGGER reject_review_transition ON queue_entries").Error; err != nil {
			t.Fatal(err)
		}
	}
	reject()
	if r := hostCall(t, f, "POST", hostPath(auth.DemoQueueID, "/call-next"), token, true); r.status != 500 {
		t.Fatalf("injected call: %d", r.status)
	}
	if s := entryState(t, f, id); s.Status != "WAITING" || s.Version != 1 || s.CalledAt != nil || s.CalledBy != nil {
		t.Fatalf("call did not roll back: %+v", s)
	}
	restore()
	if r := hostCall(t, f, "POST", hostPath(auth.DemoQueueID, "/call-next"), token, true); r.status != 200 {
		t.Fatalf("call after recovery: %d", r.status)
	}
	reject()
	if r := hostCall(t, f, "POST", hostPath(auth.DemoQueueID, "/entries/"+id+"/complete"), token, true); r.status != 500 {
		t.Fatalf("injected complete: %d", r.status)
	}
	if s := entryState(t, f, id); s.Status != "CALLED" || s.Version != 2 || s.CompletedAt != nil || s.CompletedBy != nil {
		t.Fatalf("complete did not roll back: %+v", s)
	}
	restore()
	if r := hostCall(t, f, "POST", hostPath(auth.DemoQueueID, "/entries/"+id+"/complete"), token, true); r.status != 200 {
		t.Fatalf("complete after recovery: %d", r.status)
	}
}
