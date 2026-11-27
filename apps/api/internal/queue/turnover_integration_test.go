//go:build integration

package queue

import (
	"context"
	"fmt"
	"testing"
	"time"

	"dinequeue.local/api/internal/auth"
	"github.com/google/uuid"
)

// F-09/F-16, issue #54: group checkout and cleaning.

var hostUser = auth.User{ID: host, Role: "HOST"}

func seatedFixture(t *testing.T) (fixture, CallHoldRequest, SeatingResult) {
	t.Helper()
	f, r := seatingFixture(t)
	held := mustHold(t, f, r)
	seated, err := f.service.Seat(context.Background(), hostUser, auth.DemoQueueID, held.Assignment.ID, ActionVersion{ExpectedVersion: held.Assignment.Version, IdempotencyKey: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	return f, r, seated
}

type turnoverStep func(context.Context, auth.User, string, string, ActionVersion) (SeatingResult, error)

func steps(f fixture) map[string]turnoverStep {
	return map[string]turnoverStep{"checkout": f.service.Checkout, "start-cleaning": f.service.StartCleaning, "finish-cleaning": f.service.FinishCleaning}
}

func mustStep(t *testing.T, f fixture, name string, a Assignment) SeatingResult {
	t.Helper()
	out, err := steps(f)[name](context.Background(), hostUser, auth.DemoQueueID, a.ID, ActionVersion{ExpectedVersion: a.Version, IdempotencyKey: uuid.NewString()})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return out
}

func tableStates(t *testing.T, f fixture, ids []string, status string) int64 {
	t.Helper()
	return countSQL(t, f, "SELECT count(*) FROM dining_tables WHERE id IN ? AND status=?", ids, status)
}

// secondQueueEntry creates another SEATING queue on the same floor with a ready
// ten-person entry, so a new hold can compete for the same physical tables.
func secondQueueEntry(t *testing.T, f fixture) (string, string) {
	t.Helper()
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
	return q2, e2
}

func holdRequest(t *testing.T, f fixture, q, entry string, ids []string) (CallHoldRequest, error) {
	t.Helper()
	p, err := f.service.Matching(context.Background(), hostUser, q, ids, "")
	if err != nil {
		return CallHoldRequest{}, err
	}
	if p.Entry == nil || len(p.Candidates) != 1 {
		return CallHoldRequest{}, matchError("TABLE_NOT_AVAILABLE")
	}
	return CallHoldRequest{EntryID: entry, TableIDs: ids, ExpectedLayoutVersion: p.LayoutVersion, ExpectedEntryVersion: *p.EntryVersion, ExpectedTableVersions: p.Candidates[0].TableVersions, IdempotencyKey: uuid.NewString(), SelectionReason: p.Candidates[0].SelectionReason}, nil
}

// S2-T-016, S2-T-024, S2-T-027: whole-group DIRTY -> CLEANING -> AVAILABLE with
// claims retained until finish, history and timestamps preserved.
func TestF09FullGroupTurnover(t *testing.T) {
	f, r, seated := seatedFixture(t)
	q2, e2 := secondQueueEntry(t, f)

	closed := mustStep(t, f, "checkout", seated.Assignment)
	if closed.Assignment.Status != "CLOSED" || closed.Assignment.TurnoverPhase == nil || *closed.Assignment.TurnoverPhase != "DIRTY" || closed.Assignment.ClosedAt == nil || closed.Entry.Status != "COMPLETED" {
		t.Fatalf("checkout %+v", closed)
	}
	if tableStates(t, f, r.TableIDs, "DIRTY") != 2 || countSQL(t, f, "SELECT count(*) FROM seating_assignment_tables WHERE assignment_id=? AND released_at IS NULL", seated.Assignment.ID) != 2 {
		t.Fatal("checkout must set every member DIRTY and keep claims")
	}
	// Dirty tables cannot be allocated to another queue's party.
	if req, err := holdRequest(t, f, q2, e2, r.TableIDs); err == nil {
		if _, err := f.service.CallHold(context.Background(), hostUser, q2, req); err == nil {
			t.Fatal("DIRTY tables were claimed")
		}
	}

	cleaning := mustStep(t, f, "start-cleaning", closed.Assignment)
	if *cleaning.Assignment.TurnoverPhase != "CLEANING" || cleaning.Assignment.CleaningStartedAt == nil || tableStates(t, f, r.TableIDs, "CLEANING") != 2 {
		t.Fatalf("start cleaning %+v", cleaning.Assignment)
	}
	if req, err := holdRequest(t, f, q2, e2, r.TableIDs); err == nil {
		if _, err := f.service.CallHold(context.Background(), hostUser, q2, req); err == nil {
			t.Fatal("CLEANING tables were claimed")
		}
	}
	if n := countSQL(t, f, "SELECT count(*) FROM seating_assignment_tables WHERE released_at IS NULL"); n != 2 {
		t.Fatalf("claims during cleaning %d", n)
	}

	done := mustStep(t, f, "finish-cleaning", cleaning.Assignment)
	a := done.Assignment
	if a.Status != "CLOSED" || *a.TurnoverPhase != "CLEANED" || a.CleanedAt == nil || a.ReleasedAt == nil || a.ReleaseReason == nil || *a.ReleaseReason != "CLEANED" {
		t.Fatalf("finish cleaning %+v", a)
	}
	if !a.SeatedAt.Equal(*seated.Assignment.SeatedAt) || !a.ClosedAt.Equal(*closed.Assignment.ClosedAt) || !a.CleaningStartedAt.Equal(*cleaning.Assignment.CleaningStartedAt) || !a.HeldAt.Equal(seated.Assignment.HeldAt) {
		t.Fatal("lifecycle timestamps changed")
	}
	if tableStates(t, f, r.TableIDs, "AVAILABLE") != 2 || len(done.Tables) != 2 {
		t.Fatal("finish must return every member to AVAILABLE")
	}
	// Historical membership is retained, only released.
	if n := countSQL(t, f, "SELECT count(*) FROM seating_assignment_tables WHERE assignment_id=? AND released_at IS NOT NULL", a.ID); n != 2 {
		t.Fatalf("historical members %d", n)
	}
	for _, action := range []string{"CHECKOUT", "START_CLEANING", "FINISH_CLEANING"} {
		if n := countSQL(t, f, "SELECT count(*) FROM operation_events WHERE assignment_id=? AND action=? AND actor_id=?", a.ID, action, host); n != 1 {
			t.Fatalf("%s events %d", action, n)
		}
	}
	snapshot, err := f.service.Floor(context.Background(), hostUser, auth.DemoQueueID)
	if err != nil {
		t.Fatal(err)
	}
	for _, listed := range snapshot.Assignments {
		if listed.ID == a.ID {
			t.Fatal("cleaned group still listed as outstanding")
		}
	}
	// Only now can the same tables be allocated again.
	req, err := holdRequest(t, f, q2, e2, r.TableIDs)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.CallHold(context.Background(), hostUser, q2, req); err != nil {
		t.Fatalf("cleaned tables should be claimable: %v", err)
	}
}

// S2-T-017, S2-T-026, S2-T-054: shortcuts and fresh-key repeats are rejected
// without changes; replay returns the committed response; changed payload conflicts.
func TestF09TransitionGuardsAndIdempotency(t *testing.T) {
	ctx := context.Background()
	f, r := seatingFixture(t)
	held := mustHold(t, f, r)
	for _, name := range []string{"checkout", "start-cleaning", "finish-cleaning"} {
		if _, err := steps(f)[name](ctx, hostUser, auth.DemoQueueID, held.Assignment.ID, ActionVersion{ExpectedVersion: held.Assignment.Version, IdempotencyKey: uuid.NewString()}); err != ErrInvalidTransition {
			t.Fatalf("%s on HELD: %v", name, err)
		}
	}
	if tableStates(t, f, r.TableIDs, "HELD") != 2 {
		t.Fatal("HELD shortcut changed tables")
	}
	seated, err := f.service.Seat(ctx, hostUser, auth.DemoQueueID, held.Assignment.ID, ActionVersion{ExpectedVersion: held.Assignment.Version, IdempotencyKey: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"start-cleaning", "finish-cleaning"} {
		if _, err := steps(f)[name](ctx, hostUser, auth.DemoQueueID, seated.Assignment.ID, ActionVersion{ExpectedVersion: seated.Assignment.Version, IdempotencyKey: uuid.NewString()}); err != ErrInvalidTransition {
			t.Fatalf("%s on ACTIVE: %v", name, err)
		}
	}
	if tableStates(t, f, r.TableIDs, "OCCUPIED") != 2 {
		t.Fatal("ACTIVE shortcut changed tables")
	}
	if _, err := f.service.Checkout(ctx, hostUser, auth.DemoQueueID, seated.Assignment.ID, ActionVersion{ExpectedVersion: seated.Assignment.Version + 3, IdempotencyKey: uuid.NewString()}); err != matchError("STALE_PREVIEW") {
		t.Fatalf("stale checkout %v", err)
	}
	key := ActionVersion{ExpectedVersion: seated.Assignment.Version, IdempotencyKey: uuid.NewString()}
	closed, err := f.service.Checkout(ctx, hostUser, auth.DemoQueueID, seated.Assignment.ID, key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.FinishCleaning(ctx, hostUser, auth.DemoQueueID, seated.Assignment.ID, ActionVersion{ExpectedVersion: closed.Assignment.Version, IdempotencyKey: uuid.NewString()}); err != ErrInvalidTransition {
		t.Fatalf("DIRTY -> AVAILABLE shortcut: %v", err)
	}
	replay, err := f.service.Checkout(ctx, hostUser, auth.DemoQueueID, seated.Assignment.ID, key)
	if err != nil || replay.Assignment.Version != closed.Assignment.Version || *replay.Assignment.TurnoverPhase != "DIRTY" {
		t.Fatalf("replay %+v %v", replay.Assignment, err)
	}
	changed := key
	changed.ExpectedVersion++
	if _, err := f.service.Checkout(ctx, hostUser, auth.DemoQueueID, seated.Assignment.ID, changed); err != matchError("IDEMPOTENCY_CONFLICT") {
		t.Fatalf("changed payload %v", err)
	}
	if _, err := f.service.Checkout(ctx, hostUser, auth.DemoQueueID, seated.Assignment.ID, ActionVersion{ExpectedVersion: closed.Assignment.Version, IdempotencyKey: uuid.NewString()}); err != ErrInvalidTransition {
		t.Fatalf("duplicate checkout with fresh key %v", err)
	}
	if n := countSQL(t, f, "SELECT count(*) FROM operation_events WHERE action='CHECKOUT'"); n != 1 {
		t.Fatalf("checkout events %d", n)
	}
	if tableStates(t, f, r.TableIDs, "DIRTY") != 2 {
		t.Fatal("rejected actions changed DIRTY tables")
	}
	// Replay still requires authorization.
	if _, err := f.service.Checkout(ctx, auth.User{ID: otherHost, Role: "HOST"}, auth.DemoQueueID, seated.Assignment.ID, key); err != auth.ErrForbidden {
		t.Fatalf("ungranted replay %v", err)
	}
	// Foreign assignment ID or another queue is not found.
	if _, err := f.service.StartCleaning(ctx, hostUser, auth.DemoQueueID, uuid.NewString(), ActionVersion{ExpectedVersion: 1, IdempotencyKey: uuid.NewString()}); err != ErrNotFound {
		t.Fatalf("unknown assignment %v", err)
	}
	q2, _ := secondQueueEntry(t, f)
	if _, err := f.service.StartCleaning(ctx, hostUser, q2, seated.Assignment.ID, ActionVersion{ExpectedVersion: closed.Assignment.Version, IdempotencyKey: uuid.NewString()}); err != ErrNotFound {
		t.Fatalf("cross-queue assignment %v", err)
	}
}

// S2-T-028-style HTTP gates for the three routes.
func TestF09TurnoverHTTPGates(t *testing.T) {
	f, _, seated := seatedFixture(t)
	hostToken := session(t, f, host)
	path := "/seating-assignments/" + seated.Assignment.ID + "/checkout"
	body := fmt.Sprintf(`{"expectedVersion":%d,"idempotencyKey":%q}`, seated.Assignment.Version, uuid.NewString())
	if res := requestSeating(t, f, path, session(t, f, otherHost), body, true); res.status != 403 {
		t.Fatalf("ungranted %d %v", res.status, res.body)
	}
	if res := requestSeating(t, f, path, session(t, f, customer1), body, true); res.status != 403 {
		t.Fatalf("customer %d %v", res.status, res.body)
	}
	if res := requestSeating(t, f, path, hostToken, body, false); res.status != 403 || errorCode(res) != "CSRF_REJECTED" {
		t.Fatalf("csrf %d %v", res.status, res.body)
	}
	for _, bad := range []string{`{}`, `{"expectedVersion":1}`, `{"expectedVersion":1,"idempotencyKey":"x"}`, body[:len(body)-1] + `,"status":"AVAILABLE"}`} {
		if res := requestSeating(t, f, path, hostToken, bad, true); res.status != 400 || errorCode(res) != "VALIDATION_ERROR" {
			t.Fatalf("invalid body %s -> %d %v", bad, res.status, res.body)
		}
	}
	if res := requestSeating(t, f, "/seating-assignments/"+seated.Assignment.ID+"/start-cleaning", hostToken, fmt.Sprintf(`{"expectedVersion":%d,"idempotencyKey":%q}`, seated.Assignment.Version, uuid.NewString()), true); res.status != 409 || errorCode(res) != "INVALID_TRANSITION" {
		t.Fatalf("start-cleaning on ACTIVE %d %v", res.status, res.body)
	}
	res := requestSeating(t, f, path, hostToken, body, true)
	if res.status != 200 || res.body["assignment"].(map[string]any)["turnoverPhase"] != "DIRTY" || res.body["entry"].(map[string]any)["status"] != "COMPLETED" {
		t.Fatalf("checkout %d %v", res.status, res.body)
	}
	for _, table := range res.body["tables"].([]any) {
		if table.(map[string]any)["status"] != "DIRTY" {
			t.Fatalf("member %v", table)
		}
	}
	if n := countSQL(t, f, "SELECT count(*) FROM operation_events WHERE action='CHECKOUT'"); n != 1 {
		t.Fatalf("events %d", n)
	}
}

// S2-T-048: a historical cleaned group cannot alter a later allocation of its tables.
func TestF09StaleHistoricalCleaningCannotTouchLaterGroup(t *testing.T) {
	ctx := context.Background()
	f, r, seated := seatedFixture(t)
	closed := mustStep(t, f, "checkout", seated.Assignment)
	cleaning := mustStep(t, f, "start-cleaning", closed.Assignment)
	done := mustStep(t, f, "finish-cleaning", cleaning.Assignment)
	q2, e2 := secondQueueEntry(t, f)
	req, err := holdRequest(t, f, q2, e2, r.TableIDs)
	if err != nil {
		t.Fatal(err)
	}
	b, err := f.service.CallHold(ctx, hostUser, q2, req)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"checkout", "start-cleaning", "finish-cleaning"} {
		for _, version := range []int64{done.Assignment.Version, cleaning.Assignment.Version} {
			if _, err := steps(f)[name](ctx, hostUser, auth.DemoQueueID, done.Assignment.ID, ActionVersion{ExpectedVersion: version, IdempotencyKey: uuid.NewString()}); err != ErrInvalidTransition {
				t.Fatalf("stale %s v%d: %v", name, version, err)
			}
		}
	}
	if tableStates(t, f, r.TableIDs, "HELD") != 2 {
		t.Fatal("later group's tables changed")
	}
	if n := countSQL(t, f, "SELECT count(*) FROM seating_assignment_tables WHERE assignment_id=? AND released_at IS NULL", b.Assignment.ID); n != 2 {
		t.Fatal("later group's claims changed")
	}
	if n := countSQL(t, f, "SELECT count(*) FROM seating_assignments WHERE id=? AND turnover_phase='CLEANED'", done.Assignment.ID); n != 1 {
		t.Fatal("historical turnover changed")
	}
}

// S2-T-047: failure of a member update or the event rolls back the whole step.
func TestF09RollbackOnMemberOrEventFailure(t *testing.T) {
	type stage struct {
		name, prepare, from, to string
	}
	for _, s := range []stage{{"checkout", "", "OCCUPIED", "DIRTY"}, {"start-cleaning", "checkout", "DIRTY", "CLEANING"}, {"finish-cleaning", "start-cleaning", "CLEANING", "AVAILABLE"}} {
		for _, inject := range []string{"member", "event"} {
			t.Run(s.name+"/"+inject, func(t *testing.T) {
				f, r, seated := seatedFixture(t)
				a := seated.Assignment
				if s.prepare != "" {
					a = mustStep(t, f, "checkout", a).Assignment
					if s.prepare == "start-cleaning" {
						a = mustStep(t, f, "start-cleaning", a).Assignment
					}
				}
				events := countSQL(t, f, "SELECT count(*) FROM operation_events")
				keys := countSQL(t, f, "SELECT count(*) FROM operation_requests")
				claims := countSQL(t, f, "SELECT count(*) FROM seating_assignment_tables WHERE released_at IS NULL")
				sqlText := `CREATE FUNCTION reject_event() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected test failure'; END $$; CREATE TRIGGER reject_event BEFORE INSERT ON operation_events FOR EACH ROW EXECUTE FUNCTION reject_event()`
				if inject == "member" {
					sqlText = fmt.Sprintf(`CREATE FUNCTION reject_second() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF EXISTS (SELECT 1 FROM dining_tables WHERE status='%s') THEN RAISE EXCEPTION 'second-member test failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_second BEFORE UPDATE ON dining_tables FOR EACH ROW WHEN (NEW.status='%s') EXECUTE FUNCTION reject_second()`, s.to, s.to)
					if s.to == "AVAILABLE" {
						// Other restaurant tables are already AVAILABLE; reject on the second member of this group.
						sqlText = fmt.Sprintf(`CREATE FUNCTION reject_second() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF EXISTS (SELECT 1 FROM dining_tables WHERE id IN ('%s','%s') AND status='AVAILABLE') THEN RAISE EXCEPTION 'second-member test failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_second BEFORE UPDATE ON dining_tables FOR EACH ROW WHEN (NEW.status='AVAILABLE') EXECUTE FUNCTION reject_second()`, r.TableIDs[0], r.TableIDs[1])
					}
				}
				if err := f.db.Exec(sqlText).Error; err != nil {
					t.Fatal(err)
				}
				if _, err := steps(f)[s.name](context.Background(), hostUser, auth.DemoQueueID, a.ID, ActionVersion{ExpectedVersion: a.Version, IdempotencyKey: uuid.NewString()}); err == nil {
					t.Fatal("failure injection ignored")
				}
				if tableStates(t, f, r.TableIDs, s.from) != 2 {
					t.Fatal("mixed member states")
				}
				if n := countSQL(t, f, "SELECT count(*) FROM seating_assignments WHERE id=? AND version=?", a.ID, a.Version); n != 1 {
					t.Fatal("assignment changed")
				}
				if countSQL(t, f, "SELECT count(*) FROM operation_events") != events || countSQL(t, f, "SELECT count(*) FROM operation_requests") != keys {
					t.Fatal("partial event or key")
				}
				if countSQL(t, f, "SELECT count(*) FROM seating_assignment_tables WHERE released_at IS NULL") != claims {
					t.Fatal("partial claim release")
				}
			})
		}
	}
}

// Checkout/cleaning race a new hold for the same tables from another queue: a new
// hold never succeeds before finish-cleaning commits, and the state is never mixed.
func TestF09TurnoverRacesNewHoldAcrossQueues(t *testing.T) {
	for _, stage := range []string{"checkout", "start-cleaning", "finish-cleaning"} {
		for i := 0; i < 4; i++ {
			t.Run(fmt.Sprintf("%s/%d", stage, i), func(t *testing.T) {
				f, r, seated := seatedFixture(t)
				a := seated.Assignment
				if stage != "checkout" {
					a = mustStep(t, f, "checkout", a).Assignment
				}
				if stage == "finish-cleaning" {
					a = mustStep(t, f, "start-cleaning", a).Assignment
				}
				q2, e2 := secondQueueEntry(t, f)
				// Prepare B's request as if from a preview taken once the tables are free.
				ids := r.TableIDs
				var versions []TableVersion
				if err := f.db.Raw("SELECT id AS table_id, version+? AS version FROM dining_tables WHERE id IN ? ORDER BY id", map[string]int64{"checkout": 0, "start-cleaning": 0, "finish-cleaning": 1}[stage], ids).Scan(&versions).Error; err != nil {
					t.Fatal(err)
				}
				var layout int64
				if err := f.db.Raw("SELECT layout_version FROM restaurant_floor_state LIMIT 1").Scan(&layout).Error; err != nil {
					t.Fatal(err)
				}
				bReq := CallHoldRequest{EntryID: e2, TableIDs: ids, ExpectedLayoutVersion: layout, ExpectedEntryVersion: 1, ExpectedTableVersions: versions, IdempotencyKey: uuid.NewString(), SelectionReason: "RECOMMENDED"}
				start := make(chan struct{})
				stepErr, holdErr := make(chan error, 1), make(chan error, 1)
				go func() {
					<-start
					_, err := steps(f)[stage](context.Background(), hostUser, auth.DemoQueueID, a.ID, ActionVersion{ExpectedVersion: a.Version, IdempotencyKey: uuid.NewString()})
					stepErr <- err
				}()
				go func() {
					<-start
					_, err := f.service.CallHold(context.Background(), hostUser, q2, bReq)
					holdErr <- err
				}()
				close(start)
				if err := <-stepErr; err != nil {
					t.Fatalf("%s must succeed: %v", stage, err)
				}
				h := <-holdErr
				if h != nil {
					if _, ok := h.(matchError); !ok {
						t.Fatalf("unexpected hold error %v", h)
					}
				}
				want := map[string]string{"checkout": "DIRTY", "start-cleaning": "CLEANING", "finish-cleaning": "AVAILABLE"}[stage]
				if stage != "finish-cleaning" && h == nil {
					t.Fatal("new hold claimed tables before cleaning finished")
				}
				if h == nil {
					want = "HELD"
				}
				if tableStates(t, f, ids, want) != 2 {
					t.Fatalf("mixed table state, want %s", want)
				}
				if n := countSQL(t, f, "SELECT count(*) FROM seating_assignment_tables WHERE released_at IS NULL"); (h == nil && n != 2) || (h != nil && stage == "finish-cleaning" && n != 0) || (stage != "finish-cleaning" && n != 2) {
					t.Fatalf("claims %d (hold err %v)", n, h)
				}
			})
		}
	}
}

// S2-T-053: the same key and payload sent concurrently commits one transition.
func TestF09ConcurrentIdenticalCheckoutReplay(t *testing.T) {
	f, _, seated := seatedFixture(t)
	request := ActionVersion{ExpectedVersion: seated.Assignment.Version, IdempotencyKey: uuid.NewString()}
	start := make(chan struct{})
	results := make(chan SeatingResult, 6)
	errs := make(chan error, 6)
	for i := 0; i < 6; i++ {
		go func() {
			<-start
			out, err := f.service.Checkout(context.Background(), hostUser, auth.DemoQueueID, seated.Assignment.ID, request)
			results <- out
			errs <- err
		}()
	}
	close(start)
	for i := 0; i < 6; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("replay %v", err)
		}
		if out := <-results; out.Assignment.Version != seated.Assignment.Version+1 {
			t.Fatalf("response version %d", out.Assignment.Version)
		}
	}
	if n := countSQL(t, f, "SELECT count(*) FROM operation_events WHERE action='CHECKOUT'"); n != 1 {
		t.Fatalf("events %d", n)
	}
}

// Review #63: a DIRTY/CLEANING group that lost a member claim is corrupted data,
// so the step fails as an internal error (not 409) and changes nothing.
func TestF09LostClaimIsInternalError(t *testing.T) {
	for _, stage := range []string{"start-cleaning", "finish-cleaning"} {
		t.Run(stage, func(t *testing.T) {
			f, r, seated := seatedFixture(t)
			a := mustStep(t, f, "checkout", seated.Assignment).Assignment
			from := "DIRTY"
			if stage == "finish-cleaning" {
				a = mustStep(t, f, "start-cleaning", a).Assignment
				from = "CLEANING"
			}
			if err := f.db.Exec("UPDATE seating_assignment_tables SET released_at=now() WHERE assignment_id=? AND table_id=?", a.ID, r.TableIDs[0]).Error; err != nil {
				t.Fatal(err)
			}
			events := countSQL(t, f, "SELECT count(*) FROM operation_events")
			_, err := steps(f)[stage](context.Background(), hostUser, auth.DemoQueueID, a.ID, ActionVersion{ExpectedVersion: a.Version, IdempotencyKey: uuid.NewString()})
			if err != errTurnoverClaimInvariant {
				t.Fatalf("lost claim: %v", err)
			}
			if tableStates(t, f, r.TableIDs, from) != 2 || countSQL(t, f, "SELECT count(*) FROM operation_events") != events {
				t.Fatal("corrupted group changed state")
			}
			// HTTP surfaces it as a server error, never INVALID_TRANSITION.
			res := requestSeating(t, f, "/seating-assignments/"+a.ID+"/"+stage, session(t, f, host), fmt.Sprintf(`{"expectedVersion":%d,"idempotencyKey":%q}`, a.Version, uuid.NewString()), true)
			if res.status != 500 || errorCode(res) == "INVALID_TRANSITION" {
				t.Fatalf("HTTP %d %v", res.status, res.body)
			}
		})
	}
}
