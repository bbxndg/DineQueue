package queue

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

// Host handlers must not pass an empty identity to the service if a route is ever
// registered without Authenticate; they answer 401 like the other queue handlers.
func TestHostHandlersRequireIdentity(t *testing.T) {
	h := &HTTP{}
	app := fiber.New()
	app.Get("/queues", h.hostQueues)
	app.Get("/entries", h.waitingEntries)
	app.Get("/current", h.currentEntry)
	app.Post("/call-next", h.callNext)
	app.Post("/complete", h.complete)
	for _, route := range []struct{ method, path string }{
		{"GET", "/queues"}, {"GET", "/entries"}, {"GET", "/current"}, {"POST", "/call-next"}, {"POST", "/complete"},
	} {
		response, err := app.Test(httptest.NewRequest(route.method, route.path, nil))
		if err != nil {
			t.Fatalf("%s %s: %v", route.method, route.path, err)
		}
		if response.StatusCode != 401 {
			t.Errorf("%s %s without identity: %d, want 401", route.method, route.path, response.StatusCode)
		}
	}
}
