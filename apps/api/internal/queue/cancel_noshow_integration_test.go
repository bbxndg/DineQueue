//go:build integration

package queue

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"dinequeue.local/api/internal/auth"
	"github.com/google/uuid"
)

// F-08 / issue #53: customer cancellation and Host no-show.

func cancelPost(t *testing.T, f fixture, id, token, body string, csrf bool) reply {
	t.Helper()
	req := httptest.NewRequest("POST", "/api/v1/me/queue-entries/"+id+"/cancel", strings.NewReader(body))
	req.Header.Set("Origin", testOrigin)
	if token != "" {
		req.AddCookie(&http.Cookie{Name: f.auth.CookieName, Value: token})
		if csrf {
			req.Header.Set("X-CSRF-Token", auth.CSRFToken(token))
		}
	}
	return send(t, f, req)
}

func noShowBody(version int64, key string) string {
	return fmt.Sprintf(`{"expectedVersion":%d,"idempotencyKey":%q,"reasonCode":"CUSTOMER_ABSENT"}`, version, key)
}

type terminalRow struct {
	Status         string
	CancelledAt    *time.Time
	CancelledBy    *string
	NoShowAt       *time.Time
	NoShowBy       *string
	NoShowReason   *string
	Version        int64
	JoinedAt       time.Time
	SequenceNumber int
	IsReady        bool
}

func terminalState(t *testing.T, f fixture, id string) terminalRow {
	t.Helper()
	var row terminalRow
	if err := f.db.Raw("SELECT status,cancelled_at,cancelled_by,no_show_at,no_show_by,no_show_reason,version,joined_at,sequence_number,is_ready FROM queue_entries WHERE id=?", id).Scan(&row).Error; err != nil {
		t.Fatal(err)
	}
	return row
}

// S2-T-020 / S2-T-039: owner cancels WAITING regardless of readiness and in an
// inactive queue; terminal detail stays owner-readable and a rejoin gets a new
// number with the Not ready default.
func TestF08CustomerCancelOwnWaiting(t *testing.T) {
	for _, tc := range []struct {
		name     string
		ready    bool
		inactive bool
	}{{"not-ready", false, false}, {"ready", true, false}, {"inactive-queue", true, true}} {
		t.Run(tc.name, func(t *testing.T) {
			f := setup(t)
			owner := newUser(t, f, "CUSTOMER")
			token := session(t, f, owner)
			joined := join(t, f, auth.DemoQueueID, `{"partySize":3}`, token, true)
			if joined.status != 201 {
				t.Fatalf("join %d %v", joined.status, joined.body)
			}
			id := joined.body["id"].(string)
			if err := f.db.Exec("UPDATE queue_entries SET is_ready=? WHERE id=?", tc.ready, id).Error; err != nil {
				t.Fatal(err)
			}
			if tc.inactive {
				if err := f.db.Exec("UPDATE service_queues SET is_active=false WHERE id=?", auth.DemoQueueID).Error; err != nil {
					t.Fatal(err)
				}
			}
			before := terminalState(t, f, id)
			r := cancelPost(t, f, id, token, "", true)
			if r.status != 200 || r.body["status"] != "CANCELLED" || r.body["id"] != id {
				t.Fatalf("cancel %d %v", r.status, r.body)
			}
			if r.body["ticketCode"] != joined.body["ticketCode"] || r.body["joinedAt"] != joined.body["joinedAt"] || r.body["isReady"] != tc.ready {
				t.Fatalf("cancel changed history: %v vs %v", r.body, joined.body)
			}
			if r.body["queueName"] == nil || r.body["restaurantName"] == nil || r.body["timezone"] == nil {
				t.Fatalf("cancel must return the full owned Ticket: %v", r.body)
			}
			for _, leaked := range []string{"customerId", "cancelledBy", "email", "displayName"} {
				if _, ok := r.body[leaked]; ok {
					t.Fatalf("cancel leaked %s", leaked)
				}
			}
			after := terminalState(t, f, id)
			if after.Status != "CANCELLED" || after.CancelledBy == nil || *after.CancelledBy != owner || after.CancelledAt == nil ||
				after.Version != before.Version+1 || !after.JoinedAt.Equal(before.JoinedAt) || after.SequenceNumber != before.SequenceNumber {
				t.Fatalf("persisted cancel %+v (before %+v)", after, before)
			}
			if n := countSQL(t, f, "SELECT count(*) FROM operation_events WHERE entry_id=? AND action='CUSTOMER_CANCEL' AND actor_id=? AND assignment_id IS NULL AND from_state='WAITING' AND to_state='CANCELLED'", id, owner); n != 1 {
				t.Fatalf("owner event count %d", n)
			}
			if n := countSQL(t, f, "SELECT count(*) FROM operation_events"); n != 1 {
				t.Fatalf("extra events %d", n)
			}
			if got := activeTickets(t, getTicket(t, f, "active", token)); len(got) != 0 {
				t.Fatalf("cancelled ticket listed as active: %v", got)
			}
			detail := getTicket(t, f, id, token)
			if detail.status != 200 || detail.body["status"] != "CANCELLED" || detail.body["ticketCode"] != joined.body["ticketCode"] {
				t.Fatalf("owned terminal detail %d %v", detail.status, detail.body)
			}
			if tc.inactive {
				return
			}
			again := join(t, f, auth.DemoQueueID, `{"partySize":2}`, token, true)
			if again.status != 201 || again.body["id"] == id || again.body["ticketCode"] == joined.body["ticketCode"] || again.body["isReady"] != false || again.body["status"] != "WAITING" {
				t.Fatalf("rejoin %d %v", again.status, again.body)
			}
			if old := terminalState(t, f, id); old.Status != "CANCELLED" || old.Version != after.Version {
				t.Fatalf("old row reactivated %+v", old)
			}
		})
	}
}

// S2-T-021: foreign entry is a generic 404; owned wrong-state is 409; neither mutates.
func TestF08CancelOwnershipStateAndGates(t *testing.T) {
	f := setup(t)
	owner := newUser(t, f, "CUSTOMER")
	token := session(t, f, owner)
	joined := join(t, f, auth.DemoQueueID, `{"partySize":2}`, token, true)
	id := joined.body["id"].(string)
	before := terminalState(t, f, id)

	if r := cancelPost(t, f, id, session(t, f, newUser(t, f, "CUSTOMER")), "", true); r.status != 404 || errorCode(r) != "NOT_FOUND" {
		t.Fatalf("foreign cancel %d %v", r.status, r.body)
	}
	if r := cancelPost(t, f, uuid.NewString(), token, "", true); r.status != 404 || errorCode(r) != "NOT_FOUND" {
		t.Fatalf("unknown cancel %d %v", r.status, r.body)
	}
	if r := cancelPost(t, f, "not-a-uuid", token, "", true); r.status != 404 {
		t.Fatalf("malformed cancel %d %v", r.status, r.body)
	}
	if r := cancelPost(t, f, id, "", "", false); r.status != 401 {
		t.Fatalf("guest cancel %d", r.status)
	}
	if r := cancelPost(t, f, id, token, "", false); r.status != 403 || errorCode(r) != "CSRF_REJECTED" {
		t.Fatalf("missing CSRF %d %v", r.status, r.body)
	}
	if r := cancelPost(t, f, id, session(t, f, host), "", true); r.status != 403 {
		t.Fatalf("host on customer route %d", r.status)
	}
	if r := cancelPost(t, f, id, token, `{"status":"CANCELLED"}`, true); r.status != 400 || errorCode(r) != "VALIDATION_ERROR" {
		t.Fatalf("cancel body must be empty: %d %v", r.status, r.body)
	}
	if after := terminalState(t, f, id); after.Status != "WAITING" || after.Version != before.Version {
		t.Fatalf("rejected cancel mutated %+v", after)
	}

	// Owned CALLED, COMPLETED, NO_SHOW and already-CANCELLED tickets cannot be cancelled.
	hostToken := session(t, f, host)
	if err := f.db.Exec("UPDATE queue_entries SET is_ready=true WHERE id=?", id).Error; err != nil {
		t.Fatal(err)
	}
	if r := hostCall(t, f, "POST", hostPath(auth.DemoQueueID, "/call-next"), hostToken, true); r.status != 200 {
		t.Fatalf("call %d %v", r.status, r.body)
	}
	assertNoCancel := func(state string) {
		t.Helper()
		snapshot := terminalState(t, f, id)
		if r := cancelPost(t, f, id, token, "", true); r.status != 409 || errorCode(r) != "INVALID_TRANSITION" {
			t.Fatalf("%s cancel %d %v", state, r.status, r.body)
		}
		if after := terminalState(t, f, id); after.Status != state || after.Version != snapshot.Version {
			t.Fatalf("%s mutated %+v", state, after)
		}
	}
	assertNoCancel("CALLED")
	if err := f.db.Exec("UPDATE queue_entries SET status='NO_SHOW',no_show_at=called_at,no_show_by=called_by,no_show_reason='CUSTOMER_ABSENT' WHERE id=?", id).Error; err != nil {
		t.Fatal(err)
	}
	assertNoCancel("NO_SHOW")
	if err := f.db.Exec("UPDATE queue_entries SET status='COMPLETED',no_show_at=NULL,no_show_by=NULL,no_show_reason=NULL,completed_at=called_at,completed_by=called_by WHERE id=?", id).Error; err != nil {
		t.Fatal(err)
	}
	assertNoCancel("COMPLETED")

	second := join(t, f, auth.DemoQueueID, `{"partySize":2}`, token, true)
	secondID := second.body["id"].(string)
	if r := cancelPost(t, f, secondID, token, "", true); r.status != 200 {
		t.Fatalf("cancel %d %v", r.status, r.body)
	}
	id = secondID
	assertNoCancel("CANCELLED")
	if n := countSQL(t, f, "SELECT count(*) FROM operation_events WHERE action='CUSTOMER_CANCEL'"); n != 1 {
		t.Fatalf("repeat cancel events %d", n)
	}
}

// S2-T-022 / S2-T-023 / S2-T-039: no-show releases the whole held group with actor
// and reason; replay is idempotent; a new key cannot repeat it; ungranted is 403.
func TestF08HostNoShowReleasesWholeGroup(t *testing.T) {
	f, r := seatingFixture(t)
	held := mustHold(t, f, r)
	hostToken := session(t, f, host)
	path := "/entries/" + r.EntryID + "/no-show"
	key := uuid.NewString()

	if res := requestSeating(t, f, path, session(t, f, otherHost), noShowBody(held.Assignment.Version, key), true); res.status != 403 {
		t.Fatalf("ungranted no-show %d %v", res.status, res.body)
	}
	if res := requestSeating(t, f, path, hostToken, noShowBody(held.Assignment.Version, key), false); res.status != 403 || errorCode(res) != "CSRF_REJECTED" {
		t.Fatalf("missing CSRF %d %v", res.status, res.body)
	}
	for _, body := range []string{
		`{"expectedVersion":1,"idempotencyKey":"` + key + `"}`,
		`{"expectedVersion":1,"idempotencyKey":"` + key + `","reasonCode":"HOLD_EXPIRED"}`,
		`{"expectedVersion":1,"idempotencyKey":"` + key + `","reasonCode":"CUSTOMER_ABSENT","note":"x"}`,
		`{"expectedVersion":0,"idempotencyKey":"` + key + `","reasonCode":"CUSTOMER_ABSENT"}`,
		`{"expectedVersion":1,"idempotencyKey":"bad","reasonCode":"CUSTOMER_ABSENT"}`,
	} {
		if res := requestSeating(t, f, path, hostToken, body, true); res.status != 400 || errorCode(res) != "VALIDATION_ERROR" {
			t.Fatalf("invalid body %s -> %d %v", body, res.status, res.body)
		}
	}
	if res := requestSeating(t, f, "/entries/"+uuid.NewString()+"/no-show", hostToken, noShowBody(1, uuid.NewString()), true); res.status != 404 {
		t.Fatalf("unknown entry %d %v", res.status, res.body)
	}
	if res := requestSeating(t, f, path, hostToken, noShowBody(held.Assignment.Version+5, uuid.NewString()), true); res.status != 409 || errorCode(res) != "STALE_PREVIEW" {
		t.Fatalf("stale version %d %v", res.status, res.body)
	}
	if n := countSQL(t, f, "SELECT count(*) FROM dining_tables WHERE id IN ? AND status='HELD'", r.TableIDs); n != 2 {
		t.Fatal("rejected no-show changed tables")
	}

	res := requestSeating(t, f, path, hostToken, noShowBody(held.Assignment.Version, key), true)
	if res.status != 200 {
		t.Fatalf("no-show %d %v", res.status, res.body)
	}
	entry := res.body["entry"].(map[string]any)
	assignment := res.body["assignment"].(map[string]any)
	tables := res.body["tables"].([]any)
	if entry["status"] != "NO_SHOW" || assignment["status"] != "RELEASED" || assignment["releaseReason"] != "CUSTOMER_ABSENT" || len(tables) != 2 {
		t.Fatalf("no-show response %v", res.body)
	}
	for _, table := range tables {
		if table.(map[string]any)["status"] != "AVAILABLE" {
			t.Fatalf("member not released %v", table)
		}
	}
	state := terminalState(t, f, r.EntryID)
	if state.Status != "NO_SHOW" || state.NoShowBy == nil || *state.NoShowBy != host || state.NoShowReason == nil || *state.NoShowReason != "CUSTOMER_ABSENT" || state.NoShowAt == nil {
		t.Fatalf("persisted no-show %+v", state)
	}
	if n := countSQL(t, f, "SELECT count(*) FROM seating_assignment_tables WHERE assignment_id=? AND released_at IS NOT NULL", held.Assignment.ID); n != 2 {
		t.Fatalf("released claims %d", n)
	}
	if n := countSQL(t, f, "SELECT count(*) FROM operation_events WHERE action='NO_SHOW' AND actor_id=? AND assignment_id=? AND from_state='HELD' AND to_state='RELEASED'", host, held.Assignment.ID); n != 1 {
		t.Fatalf("no-show event %d", n)
	}

	// Identical replay returns the committed response without new effects.
	if again := requestSeating(t, f, path, hostToken, noShowBody(held.Assignment.Version, key), true); again.status != 200 || again.body["assignment"].(map[string]any)["id"] != held.Assignment.ID {
		t.Fatalf("replay %d %v", again.status, again.body)
	}
	if res := requestSeating(t, f, path, hostToken, noShowBody(held.Assignment.Version+1, key), true); res.status != 409 || errorCode(res) != "IDEMPOTENCY_CONFLICT" {
		t.Fatalf("changed payload same key %d %v", res.status, res.body)
	}
	if res := requestSeating(t, f, path, hostToken, noShowBody(held.Assignment.Version, uuid.NewString()), true); res.status != 409 || errorCode(res) != "INVALID_TRANSITION" {
		t.Fatalf("repeat with new key %d %v", res.status, res.body)
	}
	if n := countSQL(t, f, "SELECT count(*) FROM operation_events"); n != 2 {
		t.Fatalf("events after repeats %d", n)
	}
	if n := countSQL(t, f, "SELECT count(*) FROM operation_requests WHERE operation='no-show'"); n != 1 {
		t.Fatalf("keyed results %d", n)
	}

	// The owner keeps a readable terminal row and can rejoin with a new number.
	owner := ticketOwner(t, f, r.EntryID)
	ownerToken := session(t, f, owner)
	if detail := getTicket(t, f, r.EntryID, ownerToken); detail.status != 200 || detail.body["status"] != "NO_SHOW" {
		t.Fatalf("owned no-show detail %d %v", detail.status, detail.body)
	}
	if got := activeTickets(t, getTicket(t, f, "active", ownerToken)); len(got) != 0 {
		t.Fatalf("no-show listed as active %v", got)
	}
	// waitingEntry inserts its number directly; sync the daily counter as a real join would have.
	if err := f.db.Exec("INSERT INTO queue_day_counters(queue_id,service_date,last_number) SELECT queue_id,service_date,sequence_number FROM queue_entries WHERE id=? ON CONFLICT (queue_id,service_date) DO UPDATE SET last_number=GREATEST(queue_day_counters.last_number,EXCLUDED.last_number)", r.EntryID).Error; err != nil {
		t.Fatal(err)
	}
	again := join(t, f, auth.DemoQueueID, `{"partySize":2}`, ownerToken, true)
	if again.status != 201 || again.body["id"] == r.EntryID || again.body["isReady"] != false {
		t.Fatalf("rejoin %d %v", again.status, again.body)
	}
	if after := terminalState(t, f, r.EntryID); after.Status != "NO_SHOW" || after.Version != state.Version {
		t.Fatalf("old row changed %+v", after)
	}
}

// S2-T-041: no-show cannot act on WAITING, ACTIVE (occupied), manual queues, other
// queues or an already-expired hold.
func TestF08NoShowRejectsNonHeldTargets(t *testing.T) {
	u := auth.User{ID: host, Role: "HOST"}
	ctx := context.Background()
	t.Run("waiting", func(t *testing.T) {
		f, r := seatingFixture(t)
		if _, err := f.service.NoShow(ctx, u, auth.DemoQueueID, r.EntryID, NoShowRequest{ExpectedVersion: 1, IdempotencyKey: uuid.NewString(), ReasonCode: "CUSTOMER_ABSENT"}); err != ErrInvalidTransition {
			t.Fatalf("waiting no-show %v", err)
		}
		if n := countSQL(t, f, "SELECT count(*) FROM queue_entries WHERE id=? AND status='WAITING'", r.EntryID); n != 1 {
			t.Fatal("waiting entry changed")
		}
	})
	t.Run("active", func(t *testing.T) {
		f, r := seatingFixture(t)
		held := mustHold(t, f, r)
		seated, err := f.service.Seat(ctx, u, auth.DemoQueueID, held.Assignment.ID, ActionVersion{ExpectedVersion: held.Assignment.Version, IdempotencyKey: uuid.NewString()})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.service.NoShow(ctx, u, auth.DemoQueueID, r.EntryID, NoShowRequest{ExpectedVersion: seated.Assignment.Version, IdempotencyKey: uuid.NewString(), ReasonCode: "CUSTOMER_ABSENT"}); err != ErrInvalidTransition {
			t.Fatalf("active no-show %v", err)
		}
		if n := countSQL(t, f, "SELECT count(*) FROM dining_tables WHERE id IN ? AND status='OCCUPIED'", r.TableIDs); n != 2 {
			t.Fatal("occupied tables changed")
		}
		if n := countSQL(t, f, "SELECT count(*) FROM seating_assignments WHERE id=? AND status='ACTIVE'", held.Assignment.ID); n != 1 {
			t.Fatal("active assignment changed")
		}
	})
	t.Run("other-queue", func(t *testing.T) {
		f, r := seatingFixture(t)
		mustHold(t, f, r)
		q2 := uuid.NewString()
		if err := f.db.Exec("INSERT INTO service_queues(id,restaurant_id,name,ticket_prefix,operations_mode) SELECT ?,restaurant_id,'Second seating fixture','B','SEATING' FROM service_queues WHERE id=?", q2, auth.DemoQueueID).Error; err != nil {
			t.Fatal(err)
		}
		if err := f.db.Exec("INSERT INTO operator_queue_permissions(user_id,queue_id) VALUES(?,?)", host, q2).Error; err != nil {
			t.Fatal(err)
		}
		if _, err := f.service.NoShow(ctx, u, q2, r.EntryID, NoShowRequest{ExpectedVersion: 1, IdempotencyKey: uuid.NewString(), ReasonCode: "CUSTOMER_ABSENT"}); err != ErrNotFound {
			t.Fatalf("cross-queue no-show %v", err)
		}
		if n := countSQL(t, f, "SELECT count(*) FROM dining_tables WHERE id IN ? AND status='HELD'", r.TableIDs); n != 2 {
			t.Fatal("other queue released tables")
		}
	})
	t.Run("manual-mode", func(t *testing.T) {
		f := setup(t)
		first := joinAs(t, f)
		if res := hostCall(t, f, "POST", hostPath(auth.DemoQueueID, "/call-next"), session(t, f, host), true); res.status != 200 {
			t.Fatalf("call %d", res.status)
		}
		id := first.body["id"].(string)
		if _, err := f.service.NoShow(ctx, u, auth.DemoQueueID, id, NoShowRequest{ExpectedVersion: 1, IdempotencyKey: uuid.NewString(), ReasonCode: "CUSTOMER_ABSENT"}); err != matchError("SEATING_NOT_ENABLED") {
			t.Fatalf("manual no-show %v", err)
		}
		if n := countSQL(t, f, "SELECT count(*) FROM queue_entries WHERE id=? AND status='CALLED'", id); n != 1 {
			t.Fatal("manual call changed")
		}
	})
	t.Run("expired", func(t *testing.T) {
		f, r := seatingFixture(t)
		held := mustHold(t, f, r)
		at := held.Assignment.HoldExpiresAt
		f.service.now = func() time.Time { return at }
		if _, err := f.service.NoShow(ctx, u, auth.DemoQueueID, r.EntryID, NoShowRequest{ExpectedVersion: held.Assignment.Version, IdempotencyKey: uuid.NewString(), ReasonCode: "CUSTOMER_ABSENT"}); err != matchError("HOLD_EXPIRED") {
			t.Fatalf("expired no-show %v", err)
		}
		// Shared cleanup committed independently: system NO_SHOW, not a Host actor.
		state := terminalState(t, f, r.EntryID)
		if state.Status != "NO_SHOW" || state.NoShowBy != nil || *state.NoShowReason != "HOLD_EXPIRED" {
			t.Fatalf("expired state %+v", state)
		}
		if n := countSQL(t, f, "SELECT count(*) FROM operation_events WHERE action='EXPIRE_HOLD'"); n != 1 {
			t.Fatalf("expiry events %d", n)
		}
		if n := countSQL(t, f, "SELECT count(*) FROM operation_events WHERE action='NO_SHOW'"); n != 0 {
			t.Fatalf("no-show events %d", n)
		}
	})
}

// S2-T-031: no-show races Seat; exactly one whole-group outcome is committed.
func TestF08NoShowSeatRace(t *testing.T) {
	for i := 0; i < 6; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			f, r := seatingFixture(t)
			held := mustHold(t, f, r)
			u := auth.User{ID: host, Role: "HOST"}
			start := make(chan struct{})
			seatErr := make(chan error, 1)
			noShowErr := make(chan error, 1)
			go func() {
				<-start
				_, err := f.service.Seat(context.Background(), u, auth.DemoQueueID, held.Assignment.ID, ActionVersion{ExpectedVersion: held.Assignment.Version, IdempotencyKey: uuid.NewString()})
				seatErr <- err
			}()
			go func() {
				<-start
				_, err := f.service.NoShow(context.Background(), u, auth.DemoQueueID, r.EntryID, NoShowRequest{ExpectedVersion: held.Assignment.Version, IdempotencyKey: uuid.NewString(), ReasonCode: "CUSTOMER_ABSENT"})
				noShowErr <- err
			}()
			close(start)
			s, n := <-seatErr, <-noShowErr
			if (s == nil) == (n == nil) {
				t.Fatalf("exactly one must win: seat=%v noShow=%v", s, n)
			}
			for _, err := range []error{s, n} {
				if err != nil && err != ErrInvalidTransition {
					t.Fatalf("unexpected losing error %v", err)
				}
			}
			tables, ticket, assignment := "OCCUPIED", "COMPLETED", "ACTIVE"
			if n == nil {
				tables, ticket, assignment = "AVAILABLE", "NO_SHOW", "RELEASED"
			}
			if c := countSQL(t, f, "SELECT count(*) FROM dining_tables WHERE id IN ? AND status=?", r.TableIDs, tables); c != 2 {
				t.Fatal("mixed member states")
			}
			if c := countSQL(t, f, "SELECT count(*) FROM queue_entries WHERE id=? AND status=?", r.EntryID, ticket); c != 1 {
				t.Fatal("contradictory ticket")
			}
			if c := countSQL(t, f, "SELECT count(*) FROM seating_assignments WHERE id=? AND status=?", held.Assignment.ID, assignment); c != 1 {
				t.Fatal("contradictory assignment")
			}
			if c := countSQL(t, f, "SELECT count(*) FROM operation_events"); c != 2 {
				t.Fatalf("events %d", c)
			}
		})
	}
}

// S2-T-040: owner cancellation races call+hold; exactly one wins with no partial claim.
func TestF08CancelCallHoldRace(t *testing.T) {
	for i := 0; i < 6; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			f, r := seatingFixture(t)
			owner := auth.User{ID: ticketOwner(t, f, r.EntryID), Role: "CUSTOMER"}
			counters := countSQL(t, f, "SELECT coalesce(sum(last_number),0) FROM queue_day_counters")
			start := make(chan struct{})
			holdErr := make(chan error, 1)
			cancelErr := make(chan error, 1)
			go func() {
				<-start
				_, err := f.service.CallHold(context.Background(), auth.User{ID: host, Role: "HOST"}, auth.DemoQueueID, r)
				holdErr <- err
			}()
			go func() {
				<-start
				_, err := f.service.Cancel(context.Background(), owner, r.EntryID)
				cancelErr <- err
			}()
			close(start)
			h, c := <-holdErr, <-cancelErr
			if (h == nil) == (c == nil) {
				t.Fatalf("exactly one must win: hold=%v cancel=%v", h, c)
			}
			if c == nil {
				if _, ok := h.(matchError); !ok {
					t.Fatalf("hold loser error %v", h)
				}
				if n := countSQL(t, f, "SELECT count(*) FROM queue_entries WHERE id=? AND status='CANCELLED' AND called_at IS NULL", r.EntryID); n != 1 {
					t.Fatal("cancel winner not persisted")
				}
				if n := countSQL(t, f, "SELECT count(*) FROM dining_tables WHERE id IN ? AND status='AVAILABLE'", r.TableIDs); n != 2 {
					t.Fatal("partial table claim")
				}
				if n := countSQL(t, f, "SELECT count(*) FROM seating_assignments") + countSQL(t, f, "SELECT count(*) FROM seating_assignment_tables"); n != 0 {
					t.Fatal("orphan assignment")
				}
			} else {
				if c != ErrInvalidTransition {
					t.Fatalf("cancel loser error %v", c)
				}
				if n := countSQL(t, f, "SELECT count(*) FROM queue_entries WHERE id=? AND status='CALLED' AND cancelled_at IS NULL", r.EntryID); n != 1 {
					t.Fatal("hold winner not persisted")
				}
				if n := countSQL(t, f, "SELECT count(*) FROM dining_tables WHERE id IN ? AND status='HELD'", r.TableIDs); n != 2 {
					t.Fatal("partial hold")
				}
			}
			if n := countSQL(t, f, "SELECT count(*) FROM operation_events"); n != 1 {
				t.Fatalf("events %d", n)
			}
			if n := countSQL(t, f, "SELECT coalesce(sum(last_number),0) FROM queue_day_counters"); n != counters {
				t.Fatal("counter changed")
			}
		})
	}
}

// Failure injection: the event insert fails, so no partial state, claim, key or event commits.
func TestF08RollbackOnEventFailure(t *testing.T) {
	inject := func(t *testing.T, f fixture) {
		t.Helper()
		if err := f.db.Exec(`CREATE FUNCTION reject_event() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected test failure'; END $$; CREATE TRIGGER reject_event BEFORE INSERT ON operation_events FOR EACH ROW EXECUTE FUNCTION reject_event()`).Error; err != nil {
			t.Fatal(err)
		}
	}
	t.Run("cancel", func(t *testing.T) {
		f := setup(t)
		owner := newUser(t, f, "CUSTOMER")
		token := session(t, f, owner)
		id := join(t, f, auth.DemoQueueID, `{"partySize":2}`, token, true).body["id"].(string)
		before := terminalState(t, f, id)
		inject(t, f)
		if res := cancelPost(t, f, id, token, "", true); res.status != 500 {
			t.Fatalf("injected cancel %d %v", res.status, res.body)
		}
		if after := terminalState(t, f, id); after.Status != "WAITING" || after.Version != before.Version || after.CancelledAt != nil {
			t.Fatalf("partial cancel %+v", after)
		}
	})
	t.Run("no-show", func(t *testing.T) {
		f, r := seatingFixture(t)
		held := mustHold(t, f, r)
		inject(t, f)
		if _, err := f.service.NoShow(context.Background(), auth.User{ID: host, Role: "HOST"}, auth.DemoQueueID, r.EntryID, NoShowRequest{ExpectedVersion: held.Assignment.Version, IdempotencyKey: uuid.NewString(), ReasonCode: "CUSTOMER_ABSENT"}); err == nil {
			t.Fatal("failure injection ignored")
		}
		if n := countSQL(t, f, "SELECT count(*) FROM dining_tables WHERE id IN ? AND status='HELD'", r.TableIDs); n != 2 {
			t.Fatal("partial table write")
		}
		if n := countSQL(t, f, "SELECT count(*) FROM queue_entries WHERE id=? AND status='CALLED'", r.EntryID); n != 1 {
			t.Fatal("partial ticket write")
		}
		if n := countSQL(t, f, "SELECT count(*) FROM seating_assignment_tables WHERE released_at IS NULL"); n != 2 {
			t.Fatal("partial claim release")
		}
		if n := countSQL(t, f, "SELECT count(*) FROM seating_assignments WHERE status='HELD'"); n != 1 {
			t.Fatal("partial assignment release")
		}
		if n := countSQL(t, f, "SELECT count(*) FROM operation_requests WHERE operation='no-show'"); n != 0 {
			t.Fatal("partial key")
		}
		if n := countSQL(t, f, "SELECT count(*) FROM operation_events"); n != 1 {
			t.Fatal("partial event")
		}
	})
}
