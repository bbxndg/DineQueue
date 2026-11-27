//go:build integration

package queue

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"dinequeue.local/api/internal/auth"
	"github.com/google/uuid"
)

// Sprint 3 Feat 2 / issue #56: Customer Ticket History (S3-AC-01..03, H01-H07, H11 API part).

type historyReply struct {
	status int
	body   map[string]any
	cache  string
}

func historyGet(t *testing.T, f fixture, token, query string) historyReply {
	t.Helper()
	req := httptest.NewRequest("GET", "/api/v1/me/queue-entries/history"+query, nil)
	if token != "" {
		req.AddCookie(&http.Cookie{Name: f.auth.CookieName, Value: token})
	}
	response, err := f.app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	out := historyReply{status: response.StatusCode, cache: response.Header.Get("Cache-Control")}
	if err := json.NewDecoder(response.Body).Decode(&out.body); err != nil {
		t.Fatal(err)
	}
	return out
}

func historyIDs(t *testing.T, r historyReply) []string {
	t.Helper()
	list, ok := r.body["tickets"].([]any)
	if r.status != 200 || !ok {
		t.Fatalf("history %d %v", r.status, r.body)
	}
	ids := make([]string, 0, len(list))
	for _, item := range list {
		ids = append(ids, item.(map[string]any)["id"].(string))
	}
	return ids
}

func nextCursor(r historyReply) string {
	c, _ := r.body["nextCursor"].(string)
	return c
}

type historyRows struct {
	f   fixture
	seq int
}

// insert creates a lifecycle-valid row directly (legacy rows have no events).
func (h *historyRows) insert(t *testing.T, customer, status string, joinedAt time.Time) string {
	t.Helper()
	h.seq++
	id := uuid.NewString()
	at := joinedAt.Add(time.Minute)
	var err error
	base := `INSERT INTO queue_entries(id,queue_id,customer_id,service_date,sequence_number,party_size,status,joined_at,is_ready`
	switch status {
	case "WAITING":
		err = h.f.db.Exec(base+`) VALUES (?,?,?,?,?,3,'WAITING',?,false)`, id, auth.DemoQueueID, customer, joinedAt.Format(time.DateOnly), h.seq, joinedAt).Error
	case "CALLED":
		err = h.f.db.Exec(base+`,called_at,called_by) VALUES (?,?,?,?,?,3,'CALLED',?,true,?,?)`, id, auth.DemoQueueID, customer, joinedAt.Format(time.DateOnly), h.seq, joinedAt, at, host).Error
	case "COMPLETED":
		err = h.f.db.Exec(base+`,called_at,called_by,completed_at,completed_by) VALUES (?,?,?,?,?,3,'COMPLETED',?,true,?,?,?,?)`, id, auth.DemoQueueID, customer, joinedAt.Format(time.DateOnly), h.seq, joinedAt, at, host, at, host).Error
	case "CANCELLED":
		err = h.f.db.Exec(base+`,cancelled_at,cancelled_by) VALUES (?,?,?,?,?,3,'CANCELLED',?,false,?,?)`, id, auth.DemoQueueID, customer, joinedAt.Format(time.DateOnly), h.seq, joinedAt, at, customer).Error
	case "NO_SHOW":
		err = h.f.db.Exec(base+`,called_at,called_by,no_show_at,no_show_reason) VALUES (?,?,?,?,?,3,'NO_SHOW',?,true,?,?,?,'HOLD_EXPIRED')`, id, auth.DemoQueueID, customer, joinedAt.Format(time.DateOnly), h.seq, joinedAt, at, host, at).Error
	default:
		t.Fatalf("status %s", status)
	}
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func dbSnapshot(t *testing.T, f fixture) string {
	t.Helper()
	var s string
	if err := f.db.Raw("SELECT coalesce(md5(string_agg(id::text||status||version||joined_at::text||party_size||sequence_number, ',' ORDER BY id)),'') FROM queue_entries").Scan(&s).Error; err != nil {
		t.Fatal(err)
	}
	return s + fmt.Sprint("/events=", countSQL(t, f, "SELECT count(*) FROM operation_events"))
}

func rawCursor(t *testing.T, payload string) string {
	t.Helper()
	return base64.RawURLEncoding.EncodeToString([]byte(payload))
}

// H01: A sees only A's terminal rows; anonymous/expired 401, Host 403; no writes; no-store.
func TestS3HistoryOwnerAndRoleAccess(t *testing.T) {
	f := setup(t)
	rows := &historyRows{f: f}
	a, b := newUser(t, f, "CUSTOMER"), newUser(t, f, "CUSTOMER")
	now := time.Now().UTC().Truncate(time.Microsecond)
	a1 := rows.insert(t, a, "COMPLETED", now.Add(-3*time.Hour))
	a2 := rows.insert(t, a, "CANCELLED", now.Add(-2*time.Hour))
	rows.insert(t, a, "WAITING", now.Add(-time.Hour))
	rows.insert(t, b, "COMPLETED", now.Add(-90*time.Minute))
	rows.insert(t, b, "NO_SHOW", now.Add(-30*time.Minute))
	before := dbSnapshot(t, f)

	r := historyGet(t, f, session(t, f, a), "")
	if got := historyIDs(t, r); len(got) != 2 || got[0] != a2 || got[1] != a1 {
		t.Fatalf("A history %v", got)
	}
	if r.cache != "no-store" {
		t.Fatalf("Cache-Control %q", r.cache)
	}
	if r.body["nextCursor"] != nil {
		t.Fatalf("single page cursor %v", r.body["nextCursor"])
	}
	for _, leaked := range []string{"customerId", "email", "displayName", "calledBy", "completedBy"} {
		if strings.Contains(fmt.Sprint(r.body), leaked) {
			t.Fatalf("history leaked %s", leaked)
		}
	}
	if r := historyGet(t, f, "", ""); r.status != 401 || r.body["error"].(map[string]any)["code"] != "UNAUTHENTICATED" {
		t.Fatalf("anonymous %d %v", r.status, r.body)
	}
	expired := session(t, f, a)
	if err := f.db.Exec("UPDATE sessions SET created_at=now()-interval '2 hours',expires_at=now()-interval '1 minute' WHERE token_hash=?", auth.TokenHash(expired)).Error; err != nil {
		t.Fatal(err)
	}
	if r := historyGet(t, f, expired, ""); r.status != 401 {
		t.Fatalf("expired %d %v", r.status, r.body)
	}
	if r := historyGet(t, f, session(t, f, host), ""); r.status != 403 {
		t.Fatalf("host %d %v", r.status, r.body)
	}
	if dbSnapshot(t, f) != before {
		t.Fatal("history read changed tickets or events")
	}
}

// H02: foreign detail 404, owner selectors 400, a cursor derived from B never widens A's predicate.
func TestS3HistoryForeignDetailSelectorsAndCursorOwnership(t *testing.T) {
	f := setup(t)
	rows := &historyRows{f: f}
	a, b := newUser(t, f, "CUSTOMER"), newUser(t, f, "CUSTOMER")
	now := time.Now().UTC().Truncate(time.Microsecond)
	aOld := rows.insert(t, a, "COMPLETED", now.Add(-5*time.Hour))
	rows.insert(t, a, "CANCELLED", now.Add(-time.Hour))
	bMid := rows.insert(t, b, "COMPLETED", now.Add(-3*time.Hour))
	rows.insert(t, b, "CANCELLED", now.Add(-6*time.Hour))
	tokenA := session(t, f, a)
	if r := getTicket(t, f, bMid, tokenA); r.status != 404 {
		t.Fatalf("foreign detail %d", r.status)
	}
	for _, q := range []string{"?ownerId=" + b, "?userId=" + b, "?customerId=" + b, "?limit=5&ownerId=" + b} {
		if r := historyGet(t, f, tokenA, q); r.status != 400 {
			t.Fatalf("selector %s -> %d %v", q, r.status, r.body)
		}
	}
	var bJoined time.Time
	if err := f.db.Raw("SELECT joined_at FROM queue_entries WHERE id=?", bMid).Scan(&bJoined).Error; err != nil {
		t.Fatal(err)
	}
	cursor := rawCursor(t, fmt.Sprintf(`{"v":1,"joinedAt":%q,"id":%q}`, bJoined.UTC().Format("2006-01-02T15:04:05.000000Z"), bMid))
	got := historyIDs(t, historyGet(t, f, tokenA, "?cursor="+cursor))
	if len(got) != 1 || got[0] != aOld {
		t.Fatalf("B-derived cursor returned %v", got)
	}
}

// H03 + H04: exactly the three terminal states, legacy rows without events, seated COMPLETED
// with an ACTIVE meal is not relabelled; nothing is written or backfilled.
func TestS3HistoryTerminalFilterAndPreservation(t *testing.T) {
	f, r := seatingFixture(t)
	held := mustHold(t, f, r)
	if _, err := f.service.Seat(context.Background(), auth.User{ID: host, Role: "HOST"}, auth.DemoQueueID, held.Assignment.ID, ActionVersion{ExpectedVersion: held.Assignment.Version, IdempotencyKey: uuid.NewString()}); err != nil {
		t.Fatal(err)
	}
	owner := ticketOwner(t, f, r.EntryID)
	rows := &historyRows{f: f, seq: 100}
	now := time.Now().UTC().Truncate(time.Microsecond)
	legacy := rows.insert(t, owner, "COMPLETED", now.Add(-4*time.Hour))
	cancelled := rows.insert(t, owner, "CANCELLED", now.Add(-3*time.Hour))
	noShow := rows.insert(t, owner, "NO_SHOW", now.Add(-2*time.Hour))
	if err := f.db.Exec("INSERT INTO service_queues(id,restaurant_id,name,ticket_prefix) SELECT ?,restaurant_id,'Other fixture','C' FROM service_queues WHERE id=?", "10000000-0000-4000-8000-0000000000aa", auth.DemoQueueID).Error; err != nil {
		t.Fatal(err)
	}
	// Active WAITING (other queue) and CALLED (demo queue) rows must be excluded.
	waiting := rows.insert(t, owner, "WAITING", now.Add(-10*time.Minute))
	if err := f.db.Exec("UPDATE queue_entries SET queue_id='10000000-0000-4000-8000-0000000000aa' WHERE id=?", waiting).Error; err != nil {
		t.Fatal(err)
	}
	rows.insert(t, owner, "CALLED", now.Add(-5*time.Minute))
	type stored struct {
		ID, Status, ServiceDate string
		PartySize               int
		JoinedAt                time.Time
	}
	var before []stored
	if err := f.db.Raw("SELECT id,status,service_date::text AS service_date,party_size,joined_at FROM queue_entries WHERE customer_id=? ORDER BY id", owner).Scan(&before).Error; err != nil {
		t.Fatal(err)
	}
	events := countSQL(t, f, "SELECT count(*) FROM operation_events")
	snapshot := dbSnapshot(t, f)

	res := historyGet(t, f, session(t, f, owner), "")
	ids := historyIDs(t, res)
	want := map[string]bool{r.EntryID: true, legacy: true, cancelled: true, noShow: true}
	if len(ids) != 4 {
		t.Fatalf("terminal rows %v", ids)
	}
	for _, id := range ids {
		if !want[id] {
			t.Fatalf("unexpected history row %s", id)
		}
	}
	byID := map[string]stored{}
	for _, s := range before {
		byID[s.ID] = s
	}
	for _, item := range res.body["tickets"].([]any) {
		ticket := item.(map[string]any)
		s := byID[ticket["id"].(string)]
		joined, err := time.Parse(time.RFC3339Nano, ticket["joinedAt"].(string))
		if err != nil || !joined.Equal(s.JoinedAt) || ticket["serviceDate"] != s.ServiceDate || int(ticket["partySize"].(float64)) != s.PartySize || ticket["status"] != s.Status {
			t.Fatalf("history changed record %v vs %+v", ticket, s)
		}
		for _, key := range []string{"id", "queueId", "ticketCode", "serviceDate", "partySize", "isReady", "status", "joinedAt", "queueName", "restaurantName", "timezone"} {
			if _, ok := ticket[key]; !ok {
				t.Fatalf("Ticket DTO missing %s", key)
			}
		}
		if _, ok := ticket["checkedOut"]; ok {
			t.Fatal("history must not infer checkout")
		}
	}
	if n := countSQL(t, f, "SELECT count(*) FROM seating_assignments WHERE id=? AND status='ACTIVE'", held.Assignment.ID); n != 1 {
		t.Fatal("seated meal must remain ACTIVE")
	}
	if countSQL(t, f, "SELECT count(*) FROM operation_events") != events || dbSnapshot(t, f) != snapshot {
		t.Fatal("history read wrote or backfilled data")
	}
}

// H05: stable descending joined_at/UUID traversal with equal timestamps and 1µs gaps,
// microsecond cursors, bounded pages and a null final cursor.
func TestS3HistoryKeysetPagination(t *testing.T) {
	f := setup(t)
	rows := &historyRows{f: f}
	owner := newUser(t, f, "CUSTOMER")
	base := time.Date(2026, 11, 20, 10, 0, 0, 123456000, time.UTC)
	statuses := []string{"COMPLETED", "CANCELLED", "NO_SHOW"}
	for i := 0; i < 4; i++ { // four rows at exactly the same instant
		rows.insert(t, owner, statuses[i%3], base)
	}
	for i := 1; i <= 3; i++ { // 1µs apart around the tie
		rows.insert(t, owner, statuses[i%3], base.Add(time.Duration(i)*time.Microsecond))
		rows.insert(t, owner, statuses[i%3], base.Add(-time.Duration(i)*time.Microsecond))
	}
	for i := 0; i < 50; i++ {
		rows.insert(t, owner, statuses[i%3], base.Add(-time.Duration(i+1)*time.Hour))
	}
	rows.insert(t, owner, "WAITING", base.Add(time.Hour))
	var expected []string
	if err := f.db.Raw("SELECT id FROM queue_entries WHERE customer_id=? AND status IN ('COMPLETED','CANCELLED','NO_SHOW') ORDER BY joined_at DESC, id DESC", owner).Scan(&expected).Error; err != nil {
		t.Fatal(err)
	}
	if len(expected) != 60 {
		t.Fatalf("fixture %d", len(expected))
	}
	token := session(t, f, owner)
	traverse := func(limit string) ([]string, int) {
		var all []string
		pages, cursor := 0, ""
		for {
			q := "?limit=" + limit
			if limit == "" {
				q = ""
			}
			if cursor != "" {
				if q == "" {
					q = "?cursor=" + cursor
				} else {
					q += "&cursor=" + cursor
				}
			}
			r := historyGet(t, f, token, q)
			ids := historyIDs(t, r)
			pages++
			all = append(all, ids...)
			cursor = nextCursor(r)
			if cursor == "" {
				if r.body["nextCursor"] != nil {
					t.Fatalf("final cursor must be null: %v", r.body["nextCursor"])
				}
				return all, pages
			}
			decoded, err := base64.RawURLEncoding.DecodeString(cursor)
			if err != nil || len(cursor) > 512 {
				t.Fatalf("cursor encoding %q %v", cursor, err)
			}
			var env map[string]any
			if err := json.Unmarshal(decoded, &env); err != nil || env["v"] != float64(1) || len(env) != 3 {
				t.Fatalf("cursor envelope %s", decoded)
			}
			stamp := env["joinedAt"].(string)
			if !strings.Contains(stamp, ".") || len(strings.Split(strings.TrimSuffix(stamp, "Z"), ".")[1]) != 6 {
				t.Fatalf("cursor must keep microseconds: %s", stamp)
			}
			if pages > 100 {
				t.Fatal("pagination did not terminate")
			}
		}
	}
	for _, tc := range []struct {
		limit string
		pages int
	}{{"1", 60}, {"7", 9}, {"20", 3}, {"50", 2}, {"", 3}} {
		got, pages := traverse(tc.limit)
		if fmt.Sprint(got) != fmt.Sprint(expected) || pages != tc.pages {
			t.Fatalf("limit %q: pages %d (want %d), order/dup mismatch=%v", tc.limit, pages, tc.pages, fmt.Sprint(got) != fmt.Sprint(expected))
		}
	}
	if got := historyIDs(t, historyGet(t, f, token, "?limit=50")); len(got) != 50 {
		t.Fatalf("limit 50 page %d", len(got))
	}
}

// H06: invalid inputs are 400 VALIDATION_ERROR without internal detail.
func TestS3HistoryRejectsInvalidQueries(t *testing.T) {
	f := setup(t)
	rows := &historyRows{f: f}
	owner := newUser(t, f, "CUSTOMER")
	existing := rows.insert(t, owner, "COMPLETED", time.Now().Add(-time.Hour))
	token := session(t, f, owner)
	valid := func(joined string, id string) string {
		return rawCursor(t, fmt.Sprintf(`{"v":1,"joinedAt":%q,"id":%q}`, joined, id))
	}
	good := "2026-11-20T10:00:00.123456Z"
	cases := []string{
		"?limit=0", "?limit=51", "?limit=1.5", "?limit=abc", "?limit=", "?limit=-1", "?limit=+5", "?limit=05x",
		"?limit=10&limit=20", "?cursor=a&cursor=b", "?foo=1", "?LIMIT=10",
		"?cursor=", "?cursor=***", "?cursor=" + strings.Repeat("A", 513),
		"?cursor=" + rawCursor(t, `not json`),
		"?cursor=" + rawCursor(t, `{"v":2,"joinedAt":"`+good+`","id":"`+existing+`"}`),
		"?cursor=" + rawCursor(t, `{"v":1,"joinedAt":"`+good+`"}`),
		"?cursor=" + rawCursor(t, `{"v":1,"joinedAt":"`+good+`","id":"`+existing+`","owner":"x"}`),
		"?cursor=" + rawCursor(t, `{"v":1,"v":1,"joinedAt":"`+good+`","id":"`+existing+`"}`),
		"?cursor=" + valid("2026-11-20 10:00:00", existing),
		"?cursor=" + valid("2026-11-20T10:00:00Z", existing),
		"?cursor=" + valid(good, "not-a-uuid"),
		"?cursor=" + valid(good, strings.ToUpper(existing)),
		"?cursor=" + base64.URLEncoding.EncodeToString([]byte(`{"v":1,"joinedAt":"`+good+`","id":"`+existing+`"}`)) + "==",
		"?cursor=" + rawCursor(t, `{"v":"1","joinedAt":"`+good+`","id":"`+existing+`"}`),
	}
	for _, q := range cases {
		r := historyGet(t, f, token, q)
		e, _ := r.body["error"].(map[string]any)
		msg := strings.ToLower(fmt.Sprint(e["message"]))
		if r.status != 400 || e["code"] != "VALIDATION_ERROR" || strings.Contains(msg, "sql") || strings.Contains(msg, "json") || strings.Contains(msg, existing) {
			t.Fatalf("%s -> %d %v", q, r.status, r.body)
		}
	}
	if r := historyGet(t, f, token, "?limit=50&cursor="+valid(good, existing)); r.status != 200 {
		t.Fatalf("valid combined query %d %v", r.status, r.body)
	}
}

// H07: a row becoming terminal above the boundary does not appear in the ongoing traversal;
// a fresh first page shows it. No snapshot guarantee is implied.
func TestS3HistoryNewTerminalRowBetweenPages(t *testing.T) {
	f := setup(t)
	rows := &historyRows{f: f}
	owner := newUser(t, f, "CUSTOMER")
	now := time.Now().UTC().Truncate(time.Microsecond)
	for i := 0; i < 4; i++ {
		rows.insert(t, owner, "COMPLETED", now.Add(-time.Duration(i+1)*time.Hour))
	}
	waiting := rows.insert(t, owner, "WAITING", now.Add(-30*time.Minute))
	token := session(t, f, owner)
	first := historyGet(t, f, token, "?limit=2")
	page1 := historyIDs(t, first)
	// The newer waiting ticket is cancelled between page requests.
	if err := f.db.Exec("UPDATE queue_entries SET status='CANCELLED',cancelled_at=now(),cancelled_by=customer_id,version=version+1 WHERE id=?", waiting).Error; err != nil {
		t.Fatal(err)
	}
	page2 := historyIDs(t, historyGet(t, f, token, "?limit=2&cursor="+nextCursor(first)))
	for _, id := range append(page1, page2...) {
		if id == waiting {
			t.Fatal("ongoing traversal must stay exclusive of rows above its boundary")
		}
	}
	if len(page2) != 2 || page2[0] == page1[1] {
		t.Fatalf("second page %v after %v", page2, page1)
	}
	fresh := historyIDs(t, historyGet(t, f, token, "?limit=2"))
	if fresh[0] != waiting {
		t.Fatalf("fresh first page %v", fresh)
	}
}

// Route order: the static history route is not captured by :entryId, and active/detail are unchanged.
func TestS3HistoryRouteDoesNotBreakOwnedReads(t *testing.T) {
	f := setup(t)
	rows := &historyRows{f: f}
	owner := newUser(t, f, "CUSTOMER")
	done := rows.insert(t, owner, "COMPLETED", time.Now().Add(-time.Hour))
	token := session(t, f, owner)
	if r := getTicket(t, f, done, token); r.status != 200 || r.body["status"] != "COMPLETED" {
		t.Fatalf("detail %d %v", r.status, r.body)
	}
	if got := activeTickets(t, getTicket(t, f, "active", token)); len(got) != 0 {
		t.Fatalf("active %v", got)
	}
	if r := historyGet(t, f, token, ""); r.status != 200 {
		t.Fatalf("history captured by detail route: %d %v", r.status, r.body)
	}
}

// Migration 0007: the history read is backed by the owner/terminal/keyset partial index.
func TestS3HistoryIndexMigration(t *testing.T) {
	f := setup(t)
	var def string
	if err := f.db.Raw("SELECT indexdef FROM pg_indexes WHERE indexname='idx_queue_entries_customer_history'").Scan(&def).Error; err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"customer_id", "joined_at DESC", "id DESC", "COMPLETED", "CANCELLED", "NO_SHOW"} {
		if !strings.Contains(def, part) {
			t.Fatalf("index definition %q lacks %s", def, part)
		}
	}
}
