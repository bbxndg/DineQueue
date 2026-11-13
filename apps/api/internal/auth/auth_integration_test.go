//go:build integration

package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	schema "dinequeue.local/api/db"
	"dinequeue.local/api/internal/health"
	"dinequeue.local/api/internal/platform/postgres"
	transport "dinequeue.local/api/internal/transport/http"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const testPassword = "F01-test-only-password"
const testOrigin = "https://dinequeue.example.test"

func TestS1AuthFailureLogOmitsSensitiveData(t *testing.T) {
	f := setup(t)
	var output bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(previous) })
	email, password, token := "private-review@example.test", "private-review-password", "private-review-session"
	data, err := json.Marshal(map[string]string{"email": email, "password": password})
	if err != nil {
		t.Fatal(err)
	}
	if r := request(t, f, "POST", "/api/v1/auth/login", string(data), "", testOrigin, ""); r.StatusCode != 401 {
		t.Fatalf("failed login: %d", r.StatusCode)
	}
	if r := request(t, f, "GET", "/api/v1/auth/me", "", token, "", ""); r.StatusCode != 401 {
		t.Fatalf("failed session: %d", r.StatusCode)
	}
	text := output.String()
	if !strings.Contains(text, "authentication rejected") || !strings.Contains(text, "session rejected") {
		t.Fatal("missing redacted authentication failure events")
	}
	for _, secret := range []string{email, password, token} {
		if strings.Contains(text, secret) {
			t.Fatal("authentication log contains request credentials")
		}
	}
}

type fixture struct {
	db      *gorm.DB
	service *Service
	handler *HTTP
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
	pool, _ := root.DB()
	t.Cleanup(func() { pool.Close() })
	name := "f01_" + strings.ReplaceAll(uuid.NewString(), "-", "")
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
	scopedPool, _ := database.DB()
	t.Cleanup(func() { scopedPool.Close() })
	if err := schema.Migrate(context.Background(), database); err != nil {
		t.Fatalf("migration failed: %v", err)
	}
	if err := SeedDemo(context.Background(), database, func(string) string { return testPassword }); err != nil {
		t.Fatal("seed failed")
	}
	service, err := NewService(NewRepository(database))
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHTTP(service, testOrigin)
	app := transport.New(health.New(postgres.NewReadiness(database)))
	handler.Register(app)
	// Test-only routes exercise the reusable gates without implementing queue features.
	app.Get("/test/host/:queueId", handler.Authenticate, handler.RequireRole("HOST"), handler.RequireQueue("queueId"), func(c fiber.Ctx) error { return c.SendStatus(204) })
	app.Post("/test/mutation", handler.Authenticate, handler.CSRF, func(c fiber.Ctx) error { return c.SendStatus(204) })
	return fixture{database, service, handler, app}
}

func request(t *testing.T, f fixture, method, path, body, token, origin, csrf string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.AddCookie(&http.Cookie{Name: f.handler.CookieName, Value: token})
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}
	response, err := f.app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { response.Body.Close() })
	return response
}

func login(t *testing.T, f fixture, email string) (string, string, User) {
	t.Helper()
	data, _ := json.Marshal(map[string]string{"email": email, "password": testPassword})
	response := request(t, f, "POST", "/api/v1/auth/login", string(data), "", testOrigin, "")
	if response.StatusCode != 200 {
		t.Fatalf("login status %d", response.StatusCode)
	}
	var body struct {
		User      User
		CSRFToken string
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	var token string
	for _, cookie := range response.Cookies() {
		if cookie.Name == f.handler.CookieName {
			token = cookie.Value
			if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" || cookie.Domain != "" || cookie.MaxAge != 43200 {
				t.Fatal("unsafe session cookie")
			}
		}
	}
	if TokenHash(token) == "" || !ValidCSRF(token, body.CSRFToken) {
		t.Fatal("invalid session/CSRF response")
	}
	return token, body.CSRFToken, body.User
}

func TestF01LoginMeLogoutAndRotation(t *testing.T) {
	f := setup(t)
	token, csrf, user := login(t, f, "CUSTOMER1@example.test")
	if user.Role != "CUSTOMER" {
		t.Fatal("incorrect role")
	}
	var stored struct {
		TokenHash            string
		CreatedAt, ExpiresAt time.Time
	}
	if err := f.db.Table("sessions").Where("token_hash = ?", TokenHash(token)).Take(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.TokenHash == token || stored.ExpiresAt.Sub(stored.CreatedAt) != 12*time.Hour {
		t.Fatal("session hash/expiry contract failed")
	}
	response := request(t, f, "GET", "/api/v1/auth/me", "", token, "", "")
	if response.StatusCode != 200 {
		t.Fatal("persistent identity failed")
	}
	var body map[string]json.RawMessage
	json.NewDecoder(response.Body).Decode(&body)
	text, _ := json.Marshal(body)
	if strings.Contains(string(text), "password") || strings.Contains(string(text), token) {
		t.Fatal("sensitive identity response")
	}
	data, _ := json.Marshal(map[string]string{"email": "customer1@example.test", "password": testPassword})
	rotated := request(t, f, "POST", "/api/v1/auth/login", string(data), token, testOrigin, "")
	if rotated.StatusCode != 200 {
		t.Fatal("rotation failed")
	}
	var newToken string
	for _, cookie := range rotated.Cookies() {
		if cookie.Name == f.handler.CookieName {
			newToken = cookie.Value
		}
	}
	if newToken == token || newToken == "" {
		t.Fatal("session was not rotated")
	}
	if request(t, f, "GET", "/api/v1/auth/me", "", token, "", "").StatusCode != 401 {
		t.Fatal("old session remains usable")
	}
	if request(t, f, "POST", "/api/v1/auth/logout", "", newToken, testOrigin, CSRFToken(newToken)).StatusCode != 204 {
		t.Fatal("logout failed")
	}
	if request(t, f, "GET", "/api/v1/auth/me", "", newToken, "", "").StatusCode != 401 {
		t.Fatal("revoked session accepted")
	}
	if request(t, f, "POST", "/api/v1/auth/logout", "", token, testOrigin, csrf).StatusCode != 401 {
		t.Fatal("post-logout request accepted")
	}
}

func TestF01InvalidDisabledAndExpiredIdentity(t *testing.T) {
	f := setup(t)
	for _, email := range []string{"missing@example.test", "customer1@example.test"} {
		data, _ := json.Marshal(map[string]string{"email": email, "password": "wrong-password"})
		response := request(t, f, "POST", "/api/v1/auth/login", string(data), "", testOrigin, "")
		if response.StatusCode != 401 || len(response.Cookies()) != 0 {
			t.Fatal("bad credentials issued a session")
		}
	}
	token, _, user := login(t, f, "customer1@example.test")
	disabled := f.db.Table("users").Where("id = ?", user.ID).Update("is_active", false)
	if disabled.Error != nil || disabled.RowsAffected != 1 {
		t.Fatal("could not disable exactly one test account")
	}
	if request(t, f, "GET", "/api/v1/auth/me", "", token, "", "").StatusCode != 401 {
		t.Fatal("disabled user retained session access")
	}
	data, _ := json.Marshal(map[string]string{"email": "customer1@example.test", "password": testPassword})
	if request(t, f, "POST", "/api/v1/auth/login", string(data), "", testOrigin, "").StatusCode != 401 {
		t.Fatal("disabled account logged in")
	}
	reactivated := f.db.Table("users").Where("id = ?", user.ID).Update("is_active", true)
	if reactivated.Error != nil || reactivated.RowsAffected != 1 {
		t.Fatal("could not reactivate exactly one test account")
	}
	if request(t, f, "GET", "/api/v1/auth/me", "", token, "", "").StatusCode != 200 {
		t.Fatal("reactivated account must have a valid session before testing expiry")
	}
	f.service.now = func() time.Time { return time.Now().Add(13 * time.Hour) }
	if request(t, f, "GET", "/api/v1/auth/me", "", token, "", "").StatusCode != 401 {
		t.Fatal("expired session accepted")
	}
	if request(t, f, "GET", "/api/v1/auth/me", "", "forged-token", "", "").StatusCode != 401 {
		t.Fatal("forged session accepted")
	}
}

func TestF01RoleQueueAndOwnershipGuards(t *testing.T) {
	f := setup(t)
	customer, _, c1 := login(t, f, "customer1@example.test")
	_, _, c2 := login(t, f, "customer2@example.test")
	host, _, hostUser := login(t, f, "host@example.test")
	hostMe := request(t, f, "GET", "/api/v1/auth/me", "", host, "", "")
	if hostMe.StatusCode != 200 {
		t.Fatalf("Host current-user status %d, want 200", hostMe.StatusCode)
	}
	var identity struct {
		User User `json:"user"`
	}
	if err := json.NewDecoder(hostMe.Body).Decode(&identity); err != nil {
		t.Fatal("cannot decode Host current-user response")
	}
	if identity.User.ID != hostUser.ID || identity.User.Role != "HOST" {
		t.Fatal("Host current-user response has incorrect persisted identity or role")
	}
	other, _, _ := login(t, f, "other-host@example.test")
	for _, test := range []struct {
		token string
		want  int
	}{{"", 401}, {customer, 403}, {other, 403}, {host, 204}} {
		got := request(t, f, "GET", "/test/host/"+DemoQueueID, "", test.token, "", "").StatusCode
		if got != test.want {
			t.Errorf("scoped guard status %d want %d", got, test.want)
		}
	}
	if RequireOwner(c1, c2.ID) != ErrNotFound || RequireOwner(c1, c1.ID) != nil {
		t.Fatal("cross-user ownership guard failed")
	}
	if request(t, f, "GET", "/test/host/not-a-uuid", "", host, "", "").StatusCode != 404 {
		t.Fatal("malformed queue id accepted")
	}
	f.db.Exec("DELETE FROM operator_queue_permissions WHERE queue_id = ?", DemoQueueID)
	if request(t, f, "GET", "/test/host/"+DemoQueueID, "", host, "", "").StatusCode != 403 {
		t.Fatal("revoked grant remained usable")
	}
}

func TestF01OriginCSRFAndValidation(t *testing.T) {
	f := setup(t)
	token, csrf, _ := login(t, f, "customer1@example.test")
	for _, test := range []struct{ origin, csrf string }{{"", csrf}, {"https://evil.example", csrf}, {testOrigin, ""}, {testOrigin, "wrong"}} {
		if request(t, f, "POST", "/api/v1/auth/logout", "", token, test.origin, test.csrf).StatusCode != 403 {
			t.Fatal("forged logout accepted")
		}
		if request(t, f, "GET", "/api/v1/auth/me", "", token, "", "").StatusCode != 200 {
			t.Fatal("rejected logout revoked session")
		}
	}
	if request(t, f, "POST", "/test/mutation", "", token, testOrigin, csrf).StatusCode != 204 {
		t.Fatal("valid CSRF request denied")
	}
	for _, origin := range []string{"", "https://evil.example", testOrigin + ".evil"} {
		if request(t, f, "POST", "/api/v1/auth/login", `{}`, "", origin, "").StatusCode != 403 {
			t.Fatal("login origin check failed")
		}
	}
	for _, body := range []string{`{"email":"customer1@example.test","password":"x","role":"HOST"}`, `null`, `{} {}`, `{"email":10}`, fmt.Sprintf(`{"email":"a@example.test","password":"%s"}`, strings.Repeat("x", 129))} {
		if request(t, f, "POST", "/api/v1/auth/login", body, "", testOrigin, "").StatusCode != 400 {
			t.Fatal("invalid body accepted")
		}
	}
	f.handler.limiter = &loginLimiter{entries: make(map[string]attempts), now: time.Now}
	for i := 0; i < 10; i++ {
		if request(t, f, "POST", "/api/v1/auth/login", `{}`, "", testOrigin, "").StatusCode != 400 {
			t.Fatal("rate limit denied early")
		}
	}
	if request(t, f, "POST", "/api/v1/auth/login", `{}`, "", testOrigin, "").StatusCode != 429 {
		t.Fatal("HTTP login rate limit failed")
	}
}

func TestF01MigrateSeedIdempotencyAndConstraints(t *testing.T) {
	f := setup(t)
	if err := schema.Migrate(context.Background(), f.db); err != nil {
		t.Fatal("repeat migration failed")
	}
	var before string
	f.db.Table("users").Select("password_hash").Where("email = ?", "host@example.test").Scan(&before)
	if err := SeedDemo(context.Background(), f.db, func(string) string { return "different-test-only-password" }); err != nil {
		t.Fatal("repeat seed failed")
	}
	var after string
	f.db.Table("users").Select("password_hash").Where("email = ?", "host@example.test").Scan(&after)
	var count int64
	f.db.Table("users").Count(&count)
	if count != 4 || before != after {
		t.Fatal("seed changed existing accounts/passwords")
	}
	f.db.Table("operator_queue_permissions").Count(&count)
	if count != 1 {
		t.Fatal("seed grants duplicated")
	}
	// A conflicting stable seed identity must roll back the entire seed, never silently skip it.
	for _, sql := range []string{
		"INSERT INTO users(id,display_name,email,password_hash,role) SELECT gen_random_uuid(),display_name,email,password_hash,role FROM users LIMIT 1",
		"UPDATE users SET role = 'MANAGER'",
		"INSERT INTO sessions(id,user_id,token_hash,created_at,expires_at) VALUES (gen_random_uuid(),gen_random_uuid(),repeat('a',64),now(),now()+interval '1 hour')",
	} {
		if err := f.db.Exec(sql).Error; err == nil {
			t.Fatal("database constraint did not reject invalid data")
		}
	}
	if err := SeedDemo(context.Background(), f.db, func(string) string { return "" }); err == nil {
		t.Fatal("blank seed credentials accepted")
	}
	// Force a late seed conflict after an earlier account would have been inserted.
	f.db.Exec("DELETE FROM users WHERE email = ?", "customer1@example.test")
	f.db.Exec("UPDATE users SET email = ? WHERE email = ?", "changed@example.test", "other-host@example.test")
	if err := SeedDemo(context.Background(), f.db, func(string) string { return testPassword }); err == nil {
		t.Fatal("conflicting seed identity silently accepted")
	}
	f.db.Table("users").Where("email = ?", "customer1@example.test").Count(&count)
	if count != 0 {
		t.Fatal("failed seed left a partial account insert")
	}
	f.db.Exec("UPDATE schema_migrations SET checksum = repeat('0',64)")
	if err := schema.Migrate(context.Background(), f.db); err == nil {
		t.Fatal("edited applied migration was accepted")
	}
}

func TestF01LoginRotationRollback(t *testing.T) {
	f := setup(t)
	token, _, _ := login(t, f, "customer1@example.test")
	if err := f.db.Exec("CREATE FUNCTION reject_session() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected session insert failure'; END $$; CREATE TRIGGER reject_session BEFORE INSERT ON sessions FOR EACH ROW EXECUTE FUNCTION reject_session()").Error; err != nil {
		t.Fatal("cannot install failure fixture")
	}
	body, _ := json.Marshal(map[string]string{"email": "customer1@example.test", "password": testPassword})
	response := request(t, f, "POST", "/api/v1/auth/login", string(body), token, testOrigin, "")
	if response.StatusCode != 500 {
		t.Fatal("injected failure not reported")
	}
	if request(t, f, "GET", "/api/v1/auth/me", "", token, "", "").StatusCode != 200 {
		t.Fatal("failed rotation revoked previous session without replacement")
	}
	var count int64
	f.db.Table("sessions").Count(&count)
	if count != 1 {
		t.Fatal("failed rotation persisted an extra session")
	}
}
