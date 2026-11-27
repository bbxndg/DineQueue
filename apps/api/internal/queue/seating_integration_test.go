//go:build integration

package queue

import (
	"context"
	"dinequeue.local/api/internal/auth"
	"encoding/json"
	"github.com/google/uuid"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSeatingCallHoldSeat(t *testing.T) {
	f := setup(t)
	provisionTestFloor(t, f)
	if err := f.db.Exec("UPDATE service_queues SET operations_mode='SEATING' WHERE id=?", auth.DemoQueueID).Error; err != nil {
		t.Fatal(err)
	}
	id := waitingEntry(t, f, auth.DemoQueueID, 1, time.Now().Add(-time.Minute))
	if err := f.db.Exec("UPDATE queue_entries SET party_size=10 WHERE id=?", id).Error; err != nil {
		t.Fatal(err)
	}
	var ids []string
	if err := f.db.Raw("SELECT id FROM dining_tables WHERE code IN ('T4','T5') ORDER BY id").Scan(&ids).Error; err != nil {
		t.Fatal(err)
	}
	p, err := f.service.Matching(context.Background(), auth.User{ID: host, Role: "HOST"}, auth.DemoQueueID, ids, "")
	if err != nil {
		t.Fatal(err)
	}
	req := CallHoldRequest{EntryID: id, TableIDs: ids, ExpectedLayoutVersion: p.LayoutVersion, ExpectedEntryVersion: *p.EntryVersion, ExpectedTableVersions: p.Candidates[0].TableVersions, IdempotencyKey: uuid.NewString(), SelectionReason: p.Candidates[0].SelectionReason}
	result, err := f.service.CallHold(context.Background(), auth.User{ID: host, Role: "HOST"}, auth.DemoQueueID, req)
	if err != nil {
		t.Fatal(err)
	}
	if result.Assignment.Status != "HELD" || result.Entry.Status != "CALLED" || len(result.Tables) != 2 {
		t.Fatalf("hold %+v", result)
	}
	seated, err := f.service.Seat(context.Background(), auth.User{ID: host, Role: "HOST"}, auth.DemoQueueID, result.Assignment.ID, ActionVersion{ExpectedVersion: result.Assignment.Version, IdempotencyKey: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	if seated.Assignment.Status != "ACTIVE" || seated.Entry.Status != "COMPLETED" {
		t.Fatalf("seat %+v", seated)
	}
}

func seatingFixture(t *testing.T) (fixture, CallHoldRequest) {
	t.Helper()
	f := setup(t)
	provisionTestFloor(t, f)
	if err := f.db.Exec("UPDATE service_queues SET operations_mode='SEATING' WHERE id=?", auth.DemoQueueID).Error; err != nil {
		t.Fatal(err)
	}
	id := waitingEntry(t, f, auth.DemoQueueID, 1, time.Now().Add(-time.Minute))
	if err := f.db.Exec("UPDATE queue_entries SET party_size=10 WHERE id=?", id).Error; err != nil {
		t.Fatal(err)
	}
	var ids []string
	if err := f.db.Raw("SELECT id FROM dining_tables WHERE code IN ('T4','T5') ORDER BY id").Scan(&ids).Error; err != nil {
		t.Fatal(err)
	}
	p, err := f.service.Matching(context.Background(), auth.User{ID: host, Role: "HOST"}, auth.DemoQueueID, ids, "")
	if err != nil || p.Entry == nil {
		t.Fatalf("fixture %+v %v", p, err)
	}
	return f, CallHoldRequest{EntryID: id, TableIDs: ids, ExpectedLayoutVersion: p.LayoutVersion, ExpectedEntryVersion: *p.EntryVersion, ExpectedTableVersions: p.Candidates[0].TableVersions, IdempotencyKey: uuid.NewString(), SelectionReason: p.Candidates[0].SelectionReason}
}
func mustHold(t *testing.T, f fixture, r CallHoldRequest) SeatingResult {
	t.Helper()
	out, err := f.service.CallHold(context.Background(), auth.User{ID: host, Role: "HOST"}, auth.DemoQueueID, r)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func countSQL(t *testing.T, f fixture, query string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := f.db.Raw(query, args...).Scan(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}
func TestSeatingIdempotencyAndGuards(t *testing.T) {
	f, r := seatingFixture(t)
	u := auth.User{ID: host, Role: "HOST"}
	ctx := context.Background()
	held := mustHold(t, f, r)
	replay, err := f.service.CallHold(ctx, u, auth.DemoQueueID, r)
	if err != nil || replay.Assignment.ID != held.Assignment.ID {
		t.Fatalf("replay %+v %v", replay, err)
	}
	changed := r
	changed.ExpectedLayoutVersion++
	if _, err := f.service.CallHold(ctx, u, auth.DemoQueueID, changed); err != matchError("IDEMPOTENCY_CONFLICT") {
		t.Fatalf("key conflict %v", err)
	}
	if _, err := f.service.Seat(ctx, u, auth.DemoQueueID, held.Assignment.ID, ActionVersion{ExpectedVersion: 99, IdempotencyKey: uuid.NewString()}); err != matchError("STALE_PREVIEW") {
		t.Fatalf("stale assignment %v", err)
	}
	seat := ActionVersion{ExpectedVersion: held.Assignment.Version, IdempotencyKey: uuid.NewString()}
	seated, err := f.service.Seat(ctx, u, auth.DemoQueueID, held.Assignment.ID, seat)
	if err != nil {
		t.Fatal(err)
	}
	replay, err = f.service.Seat(ctx, u, auth.DemoQueueID, held.Assignment.ID, seat)
	if err != nil || replay.Assignment.Version != seated.Assignment.Version {
		t.Fatalf("seat replay %v", err)
	}
	replay, err = f.service.CallHold(ctx, u, auth.DemoQueueID, r)
	if err != nil || replay.Assignment.Status != "HELD" {
		t.Fatalf("original response %v", err)
	}
	if n := countSQL(t, f, "SELECT count(*) FROM operation_events"); n != 2 {
		t.Fatalf("duplicate event %d", n)
	}
	if _, err := f.service.CallNext(ctx, u, auth.DemoQueueID); err != matchError("TABLE_SELECTION_REQUIRED") {
		t.Fatal(err)
	}
	if _, err := f.service.Complete(ctx, u, auth.DemoQueueID, r.EntryID); err != matchError("SEATING_ASSIGNMENT_REQUIRED") {
		t.Fatal(err)
	}
	if err := f.db.Exec("DELETE FROM operator_queue_permissions WHERE user_id=? AND queue_id=?", host, auth.DemoQueueID).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.CallHold(ctx, u, auth.DemoQueueID, r); err != auth.ErrForbidden {
		t.Fatalf("replay bypass %v", err)
	}
}
func TestSeatingExpiryBoundaries(t *testing.T) {
	for _, offset := range []time.Duration{-time.Microsecond, 0, time.Microsecond} {
		t.Run(offset.String(), func(t *testing.T) {
			f, r := seatingFixture(t)
			out := mustHold(t, f, r)
			deadline := out.Assignment.HoldExpiresAt
			f.service.now = func() time.Time { return deadline.Add(offset) }
			seat, err := f.service.Seat(context.Background(), auth.User{ID: host, Role: "HOST"}, auth.DemoQueueID, out.Assignment.ID, ActionVersion{ExpectedVersion: 1, IdempotencyKey: uuid.NewString()})
			if offset < 0 {
				if err != nil || seat.Assignment.Status != "ACTIVE" {
					t.Fatalf("before %v", err)
				}
			} else {
				if err != matchError("HOLD_EXPIRED") {
					t.Fatalf("expired %v", err)
				}
				if n := countSQL(t, f, "SELECT count(*) FROM dining_tables WHERE id IN ? AND status='AVAILABLE'", r.TableIDs); n != 2 {
					t.Fatal("partial release")
				}
				if n := countSQL(t, f, "SELECT count(*) FROM queue_entries WHERE id=? AND status='NO_SHOW' AND no_show_reason='HOLD_EXPIRED'", r.EntryID); n != 1 {
					t.Fatal("missing no-show")
				}
				for i := 0; i < 2; i++ {
					if _, err := f.service.ExpireHolds(context.Background(), auth.User{ID: host, Role: "HOST"}, auth.DemoQueueID, uuid.NewString()); err != nil {
						t.Fatal(err)
					}
				}
				if n := countSQL(t, f, "SELECT count(*) FROM operation_events WHERE action='EXPIRE_HOLD'"); n != 1 {
					t.Fatal("duplicate expiry")
				}
			}
		})
	}
}
func TestSeatingRollbackOnEventFailure(t *testing.T) {
	for _, operation := range []string{"call", "seat"} {
		t.Run(operation, func(t *testing.T) {
			f, r := seatingFixture(t)
			var held SeatingResult
			if operation == "seat" {
				held = mustHold(t, f, r)
			}
			if err := f.db.Exec(`CREATE FUNCTION reject_event() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected test failure'; END $$; CREATE TRIGGER reject_event BEFORE INSERT ON operation_events FOR EACH ROW EXECUTE FUNCTION reject_event()`).Error; err != nil {
				t.Fatal(err)
			}
			var err error
			if operation == "call" {
				_, err = f.service.CallHold(context.Background(), auth.User{ID: host, Role: "HOST"}, auth.DemoQueueID, r)
			} else {
				_, err = f.service.Seat(context.Background(), auth.User{ID: host, Role: "HOST"}, auth.DemoQueueID, held.Assignment.ID, ActionVersion{ExpectedVersion: 1, IdempotencyKey: uuid.NewString()})
			}
			if err == nil {
				t.Fatal("failure injection ignored")
			}
			state := "AVAILABLE"
			entry := "WAITING"
			events := int64(0)
			if operation == "seat" {
				state = "HELD"
				entry = "CALLED"
				events = 1
			}
			if n := countSQL(t, f, "SELECT count(*) FROM dining_tables WHERE id IN ? AND status=?", r.TableIDs, state); n != 2 {
				t.Fatal("partial table write")
			}
			if n := countSQL(t, f, "SELECT count(*) FROM queue_entries WHERE id=? AND status=?", r.EntryID, entry); n != 1 {
				t.Fatal("partial ticket write")
			}
			if n := countSQL(t, f, "SELECT count(*) FROM operation_requests"); n != events {
				t.Fatal("partial key")
			}
			if n := countSQL(t, f, "SELECT count(*) FROM operation_events"); n != events {
				t.Fatal("partial event")
			}
		})
	}
}
func TestSeatingConcurrentSharedTableClaims(t *testing.T) {
	f, r := seatingFixture(t)
	q2 := uuid.NewString()
	if err := f.db.Exec("INSERT INTO service_queues(id,restaurant_id,name,ticket_prefix,operations_mode) SELECT ?,restaurant_id,'Second seating fixture','B','SEATING' FROM service_queues WHERE id=?", q2, auth.DemoQueueID).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Exec("INSERT INTO operator_queue_permissions(user_id,queue_id) VALUES(?,?)", host, q2).Error; err != nil {
		t.Fatal(err)
	}
	e2 := waitingEntry(t, f, q2, 1, time.Now().Add(-time.Minute))
	if err := f.db.Exec("UPDATE queue_entries SET party_size=10 WHERE id=?", e2).Error; err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 20)
	for i := 0; i < 20; i++ {
		go func(i int) {
			<-start
			request := r
			request.IdempotencyKey = uuid.NewString()
			q := auth.DemoQueueID
			if i%2 == 1 {
				q = q2
				request.EntryID = e2
			}
			_, err := f.service.CallHold(context.Background(), auth.User{ID: host, Role: "HOST"}, q, request)
			results <- err
		}(i)
	}
	close(start)
	wins := 0
	for i := 0; i < 20; i++ {
		err := <-results
		switch err {
		case nil:
			wins++
		case ErrCallInProgress, matchError("TABLE_NOT_AVAILABLE"), matchError("STALE_PREVIEW"):
		default:
			t.Errorf("unexpected losing-request error: %v", err)
		}
	}
	if wins != 1 {
		t.Fatalf("wins %d", wins)
	}
	if n := countSQL(t, f, "SELECT count(*) FROM seating_assignment_tables WHERE released_at IS NULL"); n != 2 {
		t.Fatalf("claims %d", n)
	}
	if n := countSQL(t, f, "SELECT count(*) FROM operation_events"); n != 1 {
		t.Fatalf("events %d", n)
	}
}
func TestSeatingHTTPStrictValidationAndPermissions(t *testing.T) {
	f, r := seatingFixture(t)
	payload, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, user := range []string{"", customer1, otherHost} {
		token := ""
		if user != "" {
			token = session(t, f, user)
		}
		response := requestSeating(t, f, "/match-and-call", token, string(payload), true)
		want := 403
		if user == "" {
			want = 401
		}
		if response.status != want {
			t.Fatalf("actor %s %d", user, response.status)
		}
	}
	token := session(t, f, host)
	for _, body := range []string{`{}`, `null`, `{"idempotencyKey":null}`, `{"expectedVersion":1,"expectedVersion":2}`, `{"ExpectedVersion":1,"idempotencyKey":"` + uuid.NewString() + `"}`, `{"expectedVersion":1.2,"idempotencyKey":"` + uuid.NewString() + `"}`} {
		response := requestSeating(t, f, "/seating-assignments/"+uuid.NewString()+"/seat", token, body, true)
		if response.status != 400 {
			t.Fatalf("body %s %d", body, response.status)
		}
	}
	response := requestSeating(t, f, "/match-and-call", token, string(payload), false)
	if response.status != 403 {
		t.Fatalf("CSRF %d", response.status)
	}
	if n := countSQL(t, f, "SELECT count(*) FROM seating_assignments"); n != 0 {
		t.Fatal("invalid input wrote assignments")
	}
}

func requestSeating(t *testing.T, f fixture, path, token, body string, csrf bool) reply {
	t.Helper()
	req := httptest.NewRequest("POST", hostPath(auth.DemoQueueID, path), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Cookie", f.auth.CookieName+"="+token)
	}
	if csrf {
		req.Header.Set("Origin", testOrigin)
		req.Header.Set("X-CSRF-Token", auth.CSRFToken(token))
	}
	response, err := f.app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	out := reply{status: response.StatusCode}
	if err := json.NewDecoder(response.Body).Decode(&out.body); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestSeatingStaleOrderAndMode(t *testing.T) {
	f, r := seatingFixture(t)
	u := auth.User{ID: host, Role: "HOST"}
	ctx := context.Background()
	if err := f.db.Exec("UPDATE service_queues SET operations_mode='MANUAL' WHERE id=?", auth.DemoQueueID).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.CallHold(ctx, u, auth.DemoQueueID, r); err != matchError("SEATING_NOT_ENABLED") {
		t.Fatal(err)
	}
	if err := f.db.Exec("UPDATE service_queues SET operations_mode='SEATING' WHERE id=?", auth.DemoQueueID).Error; err != nil {
		t.Fatal(err)
	}
	var small FloorTable
	if err := f.db.Raw("SELECT id,version FROM dining_tables WHERE code='T7'").Scan(&small).Error; err != nil {
		t.Fatal(err)
	}
	undersized := r
	undersized.TableIDs = []string{small.ID}
	undersized.ExpectedTableVersions = []TableVersion{{TableID: small.ID, Version: small.Version}}
	if _, err := f.service.CallHold(ctx, u, auth.DemoQueueID, undersized); err != matchError("CAPACITY_INSUFFICIENT") {
		t.Fatalf("undersized %v", err)
	}
	stale := r
	stale.ExpectedLayoutVersion++
	if _, err := f.service.CallHold(ctx, u, auth.DemoQueueID, stale); err != matchError("STALE_PREVIEW") {
		t.Fatal(err)
	}
	earlier := waitingEntry(t, f, auth.DemoQueueID, 2, time.Now().Add(-2*time.Minute))
	if _, err := f.service.CallHold(ctx, u, auth.DemoQueueID, r); err != matchError("QUEUE_ORDER_CHANGED") {
		t.Fatal(err)
	}
	if err := f.db.Exec("UPDATE queue_entries SET is_ready=false,version=version+1 WHERE id=?", earlier).Error; err != nil {
		t.Fatal(err)
	}
	mustHold(t, f, r)
	if n := countSQL(t, f, "SELECT count(*) FROM seating_assignments"); n != 1 {
		t.Fatal("rejected actions left assignments")
	}
}
func TestSeatingExpirySeatRace(t *testing.T) {
	for _, offset := range []time.Duration{-time.Microsecond, 0} {
		t.Run(offset.String(), func(t *testing.T) {
			f, r := seatingFixture(t)
			held := mustHold(t, f, r)
			u := auth.User{ID: host, Role: "HOST"}
			at := held.Assignment.HoldExpiresAt.Add(offset)
			f.service.now = func() time.Time { return at }
			start := make(chan struct{})
			done := make(chan error, 2)
			go func() {
				<-start
				_, err := f.service.Seat(context.Background(), u, auth.DemoQueueID, held.Assignment.ID, ActionVersion{ExpectedVersion: 1, IdempotencyKey: uuid.NewString()})
				done <- err
			}()
			go func() {
				<-start
				_, err := f.service.ExpireHolds(context.Background(), u, auth.DemoQueueID, uuid.NewString())
				done <- err
			}()
			close(start)
			for i := 0; i < 2; i++ {
				err := <-done
				if err != nil && err != matchError("HOLD_EXPIRED") {
					t.Fatal(err)
				}
			}
			state := "OCCUPIED"
			ticket := "COMPLETED"
			if offset == 0 {
				state = "AVAILABLE"
				ticket = "NO_SHOW"
			}
			if n := countSQL(t, f, "SELECT count(*) FROM dining_tables WHERE id IN ? AND status=?", r.TableIDs, state); n != 2 {
				t.Fatal("mixed member states")
			}
			if n := countSQL(t, f, "SELECT count(*) FROM queue_entries WHERE id=? AND status=?", r.EntryID, ticket); n != 1 {
				t.Fatal("contradictory ticket")
			}
		})
	}
}
func TestSeatingRollbackOnSecondMember(t *testing.T) {
	f, r := seatingFixture(t)
	if err := f.db.Exec(`CREATE FUNCTION reject_second_member() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF EXISTS (SELECT 1 FROM dining_tables WHERE status='HELD') THEN RAISE EXCEPTION 'second-member test failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_second BEFORE UPDATE ON dining_tables FOR EACH ROW EXECUTE FUNCTION reject_second_member()`).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.CallHold(context.Background(), auth.User{ID: host, Role: "HOST"}, auth.DemoQueueID, r); err == nil {
		t.Fatal("injection ignored")
	}
	for _, table := range []string{"seating_assignments", "seating_assignment_tables", "operation_events", "operation_requests"} {
		if n := countSQL(t, f, "SELECT count(*) FROM "+table); n != 0 {
			t.Fatalf("partial %s", table)
		}
	}
	if n := countSQL(t, f, "SELECT count(*) FROM dining_tables WHERE id IN ? AND status='AVAILABLE' AND version=1", r.TableIDs); n != 2 {
		t.Fatal("partial member/version")
	}
}

func TestSeatingConcurrentIdenticalReplay(t *testing.T) {
	f, r := seatingFixture(t)
	start := make(chan struct{})
	results := make(chan SeatingResult, 10)
	errs := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func() {
			<-start
			out, err := f.service.CallHold(context.Background(), auth.User{ID: host, Role: "HOST"}, auth.DemoQueueID, r)
			results <- out
			errs <- err
		}()
	}
	close(start)
	id := ""
	for i := 0; i < 10; i++ {
		out := <-results
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
		if id == "" {
			id = out.Assignment.ID
		}
		if id != out.Assignment.ID {
			t.Fatal("different replay assignment")
		}
	}
	for _, table := range []string{"seating_assignments", "operation_events", "operation_requests"} {
		if n := countSQL(t, f, "SELECT count(*) FROM "+table); n != 1 {
			t.Fatalf("duplicate %s", table)
		}
	}
}
func TestSeatingAssignmentsStayQueueScoped(t *testing.T) {
	f, r := seatingFixture(t)
	held := mustHold(t, f, r)
	q2 := uuid.NewString()
	if err := f.db.Exec("INSERT INTO service_queues(id,restaurant_id,name,ticket_prefix,operations_mode) SELECT ?,restaurant_id,'Private queue','B','SEATING' FROM service_queues WHERE id=?", q2, auth.DemoQueueID).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Exec("INSERT INTO operator_queue_permissions(user_id,queue_id) VALUES(?,?)", host, q2).Error; err != nil {
		t.Fatal(err)
	}
	floor, err := f.service.Floor(context.Background(), auth.User{ID: host, Role: "HOST"}, q2)
	if err != nil {
		t.Fatal(err)
	}
	if len(floor.Assignments) != 0 {
		t.Fatal("foreign assignment exposed")
	}
	busy := 0
	for _, zone := range floor.Zones {
		for _, table := range zone.Tables {
			if table.Status == "HELD" {
				busy++
			}
		}
	}
	if busy != 2 {
		t.Fatal("shared physical state missing")
	}
	if _, err := f.service.Seat(context.Background(), auth.User{ID: host, Role: "HOST"}, q2, held.Assignment.ID, ActionVersion{ExpectedVersion: 1, IdempotencyKey: uuid.NewString()}); err != ErrNotFound {
		t.Fatalf("foreign assignment %v", err)
	}
}
