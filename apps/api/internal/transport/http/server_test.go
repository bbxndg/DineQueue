package http

import (
	"context"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"dinequeue.local/api/internal/health"
	"github.com/gofiber/fiber/v3"
)

type testProbe struct {
	err   error
	calls int
}

func TestPanicReturnsGenericError(t *testing.T) {
	app := New(health.New(&testProbe{}))
	app.Get("/test-panic", func(c fiber.Ctx) error { panic("private database details") })
	res, err := app.Test(httptest.NewRequest("GET", "/test-panic", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != 500 || !strings.Contains(string(body), "INTERNAL_ERROR") {
		t.Fatalf("unexpected error response: %s", body)
	}
	if strings.Contains(string(body), "private") {
		t.Fatal("panic details leaked")
	}
}

func (p *testProbe) Ping(ctx context.Context) error {
	p.calls++
	if _, ok := ctx.Deadline(); !ok {
		return errors.New("probe has no timeout")
	}
	return p.err
}

func TestLivenessDoesNotRequireDatabase(t *testing.T) {
	probe := &testProbe{err: errors.New("database unavailable")}
	app := New(health.New(probe))
	res, err := app.Test(httptest.NewRequest("GET", "/health/live", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 || probe.calls != 0 {
		t.Fatal("liveness depends on database")
	}
	if res.Header.Get("X-Request-ID") == "" {
		t.Fatal("missing request ID")
	}
}

func TestReadinessReportsDatabaseStateWithoutLeakingErrors(t *testing.T) {
	for _, available := range []bool{true, false} {
		probe := &testProbe{}
		want := 200
		if !available {
			probe.err = errors.New("postgres://local:secret@db/private")
			want = 503
		}
		app := New(health.New(probe))
		res, err := app.Test(httptest.NewRequest("GET", "/health/ready", nil))
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != want || probe.calls != 1 {
			t.Fatalf("readiness status=%d, calls=%d", res.StatusCode, probe.calls)
		}
		if strings.Contains(string(body), "secret") || strings.Contains(string(body), "postgres") {
			t.Fatal("readiness leaked error")
		}
		if res.Header.Get("Cache-Control") != "no-store" {
			t.Fatal("readiness must not be cached")
		}
	}
}

func TestUnknownRouteHasGenericJSONError(t *testing.T) {
	app := New(health.New(&testProbe{}))
	res, err := app.Test(httptest.NewRequest("POST", "/api/v1/auth/login", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != 404 || !strings.Contains(string(body), "NOT_FOUND") {
		t.Fatalf("unexpected response: %s", body)
	}
	if strings.Contains(string(body), "/api/v1/auth/login") {
		t.Fatal("error includes internal route detail")
	}
}

func TestMethodNotAllowedPreservesClientErrorStatus(t *testing.T) {
	app := New(health.New(&testProbe{}))
	res, err := app.Test(httptest.NewRequest("POST", "/health/live", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 405 {
		t.Fatalf("expected 405, got %d", res.StatusCode)
	}
}
