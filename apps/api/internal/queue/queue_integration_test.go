//go:build integration

package queue

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	schema "dinequeue.local/api/db"
	"dinequeue.local/api/internal/auth"
	"dinequeue.local/api/internal/health"
	"dinequeue.local/api/internal/platform/postgres"
	transport "dinequeue.local/api/internal/transport/http"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const testOrigin = "https://dinequeue.example.test"
const (
	customer1 = "20000000-0000-4000-8000-000000000001"
	customer2 = "20000000-0000-4000-8000-000000000002"
	host      = "20000000-0000-4000-8000-000000000003"
)

type fixture struct {
	db      *gorm.DB
	service *Service
	auth    *auth.HTTP
	app     *fiber.App
}

func setup(t *testing.T) fixture {
	t.Helper()
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Fatal("TEST_DATABASE_URL required; use an isolated PostgreSQL database")
	}
	root, err := postgres.Open(context.Background(), raw)
	if err != nil {
		t.Fatal("database open failed")
	}
	rootPool, _ := root.DB()
	t.Cleanup(func() { rootPool.Close() })
	name := "f02_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := root.Exec("CREATE SCHEMA " + name).Error; err != nil {
		t.Fatal("cannot create isolated test schema")
	}
	t.Cleanup(func() { root.Exec("DROP SCHEMA " + name + " CASCADE") })
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal("invalid test URL")
	}
	query := parsed.Query()
	query.Set("search_path", name)
	parsed.RawQuery = query.Encode()
	database, err := postgres.Open(context.Background(), parsed.String())
	if err != nil {
		t.Fatal("cannot open scoped database")
	}
	pool, _ := database.DB()
	t.Cleanup(func() { pool.Close() })
	if err := schema.Migrate(context.Background(), database); err != nil {
		t.Fatalf("migration failed: %v", err)
	}
	if err := auth.SeedDemo(context.Background(), database, func(string) string { return "F02-test-only-password" }); err != nil {
		t.Fatal("seed failed")
	}
	authService, err := auth.NewService(auth.NewRepository(database))
	if err != nil {
		t.Fatal(err)
	}
	authHTTP := auth.NewHTTP(authService, testOrigin)
	service := NewService(NewRepository(database))
	app := transport.New(health.New(postgres.NewReadiness(database)))
	authHTTP.Register(app)
	NewHTTP(service, authHTTP).Register(app)
	return fixture{database, service, authHTTP, app}
}

// session inserts a valid session directly so concurrency tests avoid 20 Argon2 logins.
func session(t *testing.T, f fixture, userID string) string {
	t.Helper()
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		t.Fatal(err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	now := time.Now().UTC()
	if err := f.db.Exec("INSERT INTO sessions(id,user_id,token_hash,created_at,expires_at) VALUES (?,?,?,?,?)", uuid.NewString(), userID, auth.TokenHash(token), now, now.Add(time.Hour)).Error; err != nil {
		t.Fatal("cannot create test session")
	}
	return token
}

type reply struct {
	status int
	body   map[string]any
}

func join(t *testing.T, f fixture, queueID, body, token string, csrf bool) reply {
	req := httptest.NewRequest("POST", "/api/v1/queues/"+queueID+"/entries", strings.NewReader(body))
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

func send(t *testing.T, f fixture, req *http.Request) reply {
	response, err := f.app.Test(req, fiber.TestConfig{Timeout: 30 * time.Second})
	if err != nil {
		t.Error(err)
		return reply{}
	}
	defer response.Body.Close()
	var body map[string]any
	json.NewDecoder(response.Body).Decode(&body)
	return reply{response.StatusCode, body}
}

func errorCode(r reply) string {
	e, _ := r.body["error"].(map[string]any)
	code, _ := e["code"].(string)
	return code
}

func count(t *testing.T, f fixture, table string) int64 {
	var n int64
	if err := f.db.Table(table).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func TestF02JoinCreatesWaitingTicket(t *testing.T) {
	f := setup(t)
	r := join(t, f, auth.DemoQueueID, `{"partySize":4}`, session(t, f, customer1), true)
	if r.status != 201 {
		t.Fatalf("T-009 status %d body %v", r.status, r.body)
	}
	if r.body["ticketCode"] != "A-001" || r.body["status"] != "WAITING" || r.body["partySize"] != float64(4) || r.body["queueId"] != auth.DemoQueueID {
		t.Fatalf("T-009 unexpected body %v", r.body)
	}
	for _, private := range []string{"customerId", "email", "displayName", "sequenceNumber"} {
		if _, ok := r.body[private]; ok {
			t.Errorf("response exposes %s", private)
		}
	}
	var row struct {
		CustomerID, Status string
		PartySize          int
		SequenceNumber     int
		ServiceDate        time.Time
		JoinedAt           time.Time
	}
	if err := f.db.Table("queue_entries").Where("id = ?", r.body["id"]).Take(&row).Error; err != nil {
		t.Fatal("T-009 entry was not persisted")
	}
	bangkok, _ := time.LoadLocation("Asia/Bangkok")
	if row.CustomerID != customer1 || row.Status != "WAITING" || row.PartySize != 4 || row.SequenceNumber != 1 ||
		row.ServiceDate.Format("2006-01-02") != r.body["serviceDate"] || row.ServiceDate.Format("2006-01-02") != row.JoinedAt.In(bangkok).Format("2006-01-02") {
		t.Fatalf("T-009 persisted row mismatch %+v body %v", row, r.body)
	}
	// T-036: 10-person party is accepted and preserved without table allocation.
	r = join(t, f, auth.DemoQueueID, `{"partySize":10}`, session(t, f, customer2), true)
	if r.status != 201 || r.body["partySize"] != float64(10) || r.body["ticketCode"] != "A-002" || r.body["status"] != "WAITING" {
		t.Fatalf("T-036 status %d body %v", r.status, r.body)
	}
	var size int
	f.db.Table("queue_entries").Select("party_size").Where("id = ?", r.body["id"]).Scan(&size)
	if size != 10 {
		t.Fatal("T-036 party size not preserved")
	}
	// Any authenticated user can list active queues, without private fields.
	req := httptest.NewRequest("GET", "/api/v1/queues", nil)
	req.AddCookie(&http.Cookie{Name: f.auth.CookieName, Value: session(t, f, host)})
	list := send(t, f, req)
	queues, _ := list.body["queues"].([]any)
	if list.status != 200 || len(queues) != 1 || queues[0].(map[string]any)["id"] != auth.DemoQueueID {
		t.Fatalf("queue list %d %v", list.status, list.body)
	}
}

func TestF02InvalidPartySizeCreatesNothing(t *testing.T) {
	f := setup(t)
	token := session(t, f, customer1)
	for _, body := range []string{`{"partySize":0}`, `{"partySize":17}`, `{"partySize":1.5}`, `{"partySize":"4"}`, `{}`, `{"partySize":4,"status":"CALLED"}`, `not json`} {
		r := join(t, f, auth.DemoQueueID, body, token, true)
		if r.status != 400 || errorCode(r) != "VALIDATION_ERROR" {
			t.Errorf("T-011/012 %s: status %d code %s", body, r.status, errorCode(r))
		}
	}
	if count(t, f, "queue_entries") != 0 || count(t, f, "queue_day_counters") != 0 {
		t.Fatal("T-011/012 invalid request wrote data")
	}
}

func TestF02DuplicateSequentialJoinConflicts(t *testing.T) {
	f := setup(t)
	token := session(t, f, customer1)
	if r := join(t, f, auth.DemoQueueID, `{"partySize":2}`, token, true); r.status != 201 {
		t.Fatalf("first join %d", r.status)
	}
	r := join(t, f, auth.DemoQueueID, `{"partySize":3}`, token, true)
	if r.status != 409 || errorCode(r) != "ACTIVE_QUEUE_EXISTS" {
		t.Fatalf("T-013 status %d code %s", r.status, errorCode(r))
	}
	if count(t, f, "queue_entries") != 1 {
		t.Fatal("T-013 duplicate row created")
	}
	var counterAfterReject int
	if err := f.db.Table("queue_day_counters").Select("last_number").Where("queue_id = ?", auth.DemoQueueID).Scan(&counterAfterReject).Error; err != nil || counterAfterReject != 1 {
		t.Fatalf("T-013 expected counter to remain 1 after rollback, got %d (err: %v)", counterAfterReject, err)
	}
	// The rejected attempt rolled back its counter increment: the next customer gets A-002.
	if r := join(t, f, auth.DemoQueueID, `{"partySize":2}`, session(t, f, customer2), true); r.body["ticketCode"] != "A-002" {
		t.Fatalf("counter gap after rejected duplicate: %v", r.body)
	}
	// A completed (inactive) entry no longer blocks a new active entry.
	if err := f.db.Exec("UPDATE queue_entries SET status='COMPLETED', called_at=now(), called_by=?, completed_at=now(), completed_by=? WHERE customer_id = ?", host, host, customer1).Error; err != nil {
		t.Fatal(err)
	}
	if r := join(t, f, auth.DemoQueueID, `{"partySize":2}`, token, true); r.status != 201 || r.body["ticketCode"] != "A-003" {
		t.Fatalf("rejoin after completion %d %v", r.status, r.body)
	}
}

func TestF02ConcurrentDuplicateJoinCreatesOne(t *testing.T) {
	f := setup(t)
	token := session(t, f, customer1)
	results := make([]reply, 20)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results[i] = join(t, f, auth.DemoQueueID, `{"partySize":2}`, token, true)
		}(i)
	}
	close(start)
	wg.Wait()
	created, conflicts := 0, 0
	for _, r := range results {
		switch {
		case r.status == 201:
			created++
		case r.status == 409 && errorCode(r) == "ACTIVE_QUEUE_EXISTS":
			conflicts++
		default:
			t.Errorf("T-014 unexpected %d %v", r.status, r.body)
		}
	}
	if created != 1 || conflicts != len(results)-1 || count(t, f, "queue_entries") != 1 {
		t.Fatalf("T-014 created=%d conflicts=%d rows=%d", created, conflicts, count(t, f, "queue_entries"))
	}
}

func TestF02ConcurrentCustomersGetUniqueNumbers(t *testing.T) {
	f := setup(t)
	tokens := make([]string, 20)
	for i := range tokens {
		id := uuid.NewString()
		if err := f.db.Exec("INSERT INTO users(id,display_name,email,password_hash,role) VALUES (?,?,?,?, 'CUSTOMER')", id, fmt.Sprintf("Load %d", i), fmt.Sprintf("load%d@example.test", i), "not-a-login-hash").Error; err != nil {
			t.Fatal(err)
		}
		tokens[i] = session(t, f, id)
	}
	results := make([]reply, len(tokens))
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range tokens {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results[i] = join(t, f, auth.DemoQueueID, `{"partySize":3}`, tokens[i], true)
		}(i)
	}
	close(start)
	wg.Wait()
	seen := map[string]bool{}
	for _, r := range results {
		code, _ := r.body["ticketCode"].(string)
		if r.status != 201 || seen[code] {
			t.Fatalf("T-015 status %d duplicate/missing %q", r.status, code)
		}
		seen[code] = true
	}
	for n := 1; n <= 20; n++ {
		if !seen[TicketCode("A", n)] {
			t.Fatalf("T-015 missing %s", TicketCode("A", n))
		}
	}
	var last int
	f.db.Table("queue_day_counters").Select("last_number").Scan(&last)
	if last != 20 || count(t, f, "queue_entries") != 20 {
		t.Fatalf("T-015 counter %d rows %d", last, count(t, f, "queue_entries"))
	}
}

func TestF02UnknownAndInactiveQueue(t *testing.T) {
	f := setup(t)
	token := session(t, f, customer1)
	for _, id := range []string{uuid.NewString(), "not-a-uuid"} {
		if r := join(t, f, id, `{"partySize":2}`, token, true); r.status != 404 || errorCode(r) != "NOT_FOUND" {
			t.Errorf("T-016 unknown queue %s: %d %s", id, r.status, errorCode(r))
		}
	}
	f.db.Exec("UPDATE service_queues SET is_active = false WHERE id = ?", auth.DemoQueueID)
	if r := join(t, f, auth.DemoQueueID, `{"partySize":2}`, token, true); r.status != 409 || errorCode(r) != "QUEUE_INACTIVE" {
		t.Fatalf("T-016 inactive queue: %d %s", r.status, errorCode(r))
	}
	req := httptest.NewRequest("GET", "/api/v1/queues", nil)
	req.AddCookie(&http.Cookie{Name: f.auth.CookieName, Value: token})
	if list := send(t, f, req); list.status != 200 || len(list.body["queues"].([]any)) != 0 {
		t.Fatalf("inactive queue listed: %v", list.body)
	}
	if count(t, f, "queue_entries") != 0 || count(t, f, "queue_day_counters") != 0 {
		t.Fatal("T-016 rejected join wrote data")
	}
}

func TestF02RoleSessionAndCSRFGates(t *testing.T) {
	f := setup(t)
	if r := join(t, f, auth.DemoQueueID, `{"partySize":2}`, "", false); r.status != 401 {
		t.Errorf("T-005 guest join %d", r.status)
	}
	if r := join(t, f, auth.DemoQueueID, `{"partySize":2}`, session(t, f, host), true); r.status != 403 || errorCode(r) != "FORBIDDEN" {
		t.Errorf("T-017 Host join %d %s", r.status, errorCode(r))
	}
	if r := join(t, f, auth.DemoQueueID, `{"partySize":2}`, session(t, f, customer1), false); r.status != 403 || errorCode(r) != "CSRF_REJECTED" {
		t.Errorf("missing CSRF %d %s", r.status, errorCode(r))
	}
	req := httptest.NewRequest("GET", "/api/v1/queues", nil)
	if r := send(t, f, req); r.status != 401 {
		t.Errorf("guest queue list %d", r.status)
	}
	// Service rechecks role independently of the HTTP gate.
	if _, err := f.service.Join(context.Background(), auth.User{ID: host, Role: "HOST"}, auth.DemoQueueID, 2); err != auth.ErrForbidden {
		t.Errorf("service accepted Host join: %v", err)
	}
	if count(t, f, "queue_entries") != 0 {
		t.Fatal("rejected joins wrote data")
	}
}

func TestF02ServiceDateUsesRestaurantTimezone(t *testing.T) {
	f := setup(t)
	bangkok, _ := time.LoadLocation("Asia/Bangkok")
	f.service.now = func() time.Time { return time.Date(2026, 11, 5, 23, 59, 0, 0, bangkok) }
	before := join(t, f, auth.DemoQueueID, `{"partySize":2}`, session(t, f, customer1), true)
	if before.status != 201 || before.body["serviceDate"] != "2026-11-05" || before.body["ticketCode"] != "A-001" {
		t.Fatalf("T-034 before midnight %v", before.body)
	}
	f.service.now = func() time.Time { return time.Date(2026, 11, 6, 0, 0, 0, 0, bangkok) }
	after := join(t, f, auth.DemoQueueID, `{"partySize":2}`, session(t, f, customer2), true)
	// 00:00 Bangkok is still 5 Nov UTC: the local date must win and numbering restarts.
	if after.status != 201 || after.body["serviceDate"] != "2026-11-06" || after.body["ticketCode"] != "A-001" {
		t.Fatalf("T-034 after midnight %v", after.body)
	}
	if count(t, f, "queue_day_counters") != 2 {
		t.Fatal("T-034 expected one counter per local service date")
	}
}

func TestF03ActiveAndDetailEntry(t *testing.T) {
	// T-018: C1 joins, then checks active list and detail.
	f := setup(t)
	token1 := session(t, f, customer1)
	r := join(t, f, auth.DemoQueueID, `{"partySize":3}`, token1, true)
	if r.status != 201 {
		t.Fatalf("T-018 join status %d body %v", r.status, r.body)
	}
	entryID, _ := r.body["id"].(string)
	ticketCode, _ := r.body["ticketCode"].(string)

	// Active list
	req := httptest.NewRequest("GET", "/api/v1/me/queue-entries/active", nil)
	req.AddCookie(&http.Cookie{Name: f.auth.CookieName, Value: token1})
	list := send(t, f, req)
	if list.status != 200 {
		t.Fatalf("T-018 active list status %d body %v", list.status, list.body)
	}
	entries, _ := list.body["entries"].([]any)
	if len(entries) != 1 {
		t.Fatalf("T-018 expected 1 active entry, got %d", len(entries))
	}
	entry0, _ := entries[0].(map[string]any)
	if entry0["id"] != entryID || entry0["ticketCode"] != ticketCode {
		t.Fatalf("T-018 active list entry mismatch: got %v", entry0)
	}
	for _, priv := range []string{"customerId", "email", "displayName", "sequenceNumber"} {
		if _, ok := entry0[priv]; ok {
			t.Errorf("T-018 active list exposes private field %s", priv)
		}
	}

	// Detail
	req2 := httptest.NewRequest("GET", "/api/v1/me/queue-entries/"+entryID, nil)
	req2.AddCookie(&http.Cookie{Name: f.auth.CookieName, Value: token1})
	detail := send(t, f, req2)
	if detail.status != 200 {
		t.Fatalf("T-018 detail status %d body %v", detail.status, detail.body)
	}
	if detail.body["id"] != entryID || detail.body["ticketCode"] != ticketCode {
		t.Fatalf("T-018 detail mismatch: got %v", detail.body)
	}
	for _, priv := range []string{"customerId", "email", "displayName", "sequenceNumber"} {
		if _, ok := detail.body[priv]; ok {
			t.Errorf("T-018 detail exposes private field %s", priv)
		}
	}
}

func TestF03EmptyActiveList(t *testing.T) {
	// T-020: C2 has no active ticket → 200 with empty entries array.
	f := setup(t)
	token2 := session(t, f, customer2)
	req := httptest.NewRequest("GET", "/api/v1/me/queue-entries/active", nil)
	req.AddCookie(&http.Cookie{Name: f.auth.CookieName, Value: token2})
	list := send(t, f, req)
	if list.status != 200 {
		t.Fatalf("T-020 status %d body %v", list.status, list.body)
	}
	entries, _ := list.body["entries"].([]any)
	if len(entries) != 0 {
		t.Fatalf("T-020 expected empty entries, got %v", entries)
	}
}

func TestF03CrossUserPrivacy(t *testing.T) {
	// T-021: guest → 401; T-008: C2 reads C1 ticket → 404.
	f := setup(t)
	token1 := session(t, f, customer1)
	r := join(t, f, auth.DemoQueueID, `{"partySize":2}`, token1, true)
	if r.status != 201 {
		t.Fatalf("setup join status %d body %v", r.status, r.body)
	}
	entryID, _ := r.body["id"].(string)

	// T-021: unauthenticated guest → 401
	req := httptest.NewRequest("GET", "/api/v1/me/queue-entries/"+entryID, nil)
	if g := send(t, f, req); g.status != 401 {
		t.Errorf("T-021 guest expected 401, got %d", g.status)
	}

	// T-008: owner-scoped authorization denies C2 without revealing C1's ticket.
	req2 := httptest.NewRequest("GET", "/api/v1/me/queue-entries/"+entryID, nil)
	req2.AddCookie(&http.Cookie{Name: f.auth.CookieName, Value: session(t, f, customer2)})
	if other := send(t, f, req2); other.status != 404 {
		t.Errorf("T-008 cross-user expected 404, got %d body %v", other.status, other.body)
	}
}
