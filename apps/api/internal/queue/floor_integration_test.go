//go:build integration

package queue

import (
	"context"
	"dinequeue.local/api/internal/auth"
	"errors"
	"testing"
)

func TestFloorAccessAndEmptySnapshot(t *testing.T) {
	f := setup(t)
	path := hostPath(auth.DemoQueueID, "/floor")
	for _, tc := range []struct {
		user   string
		status int
	}{{"", 401}, {customer1, 403}, {otherHost, 403}, {host, 200}} {
		token := ""
		if tc.user != "" {
			token = session(t, f, tc.user)
		}
		r := hostCall(t, f, "GET", path, token, false)
		if r.status != tc.status {
			t.Fatalf("actor %q: want %d got %d %v", tc.user, tc.status, r.status, r.body)
		}
		if tc.status == 403 {
			role := "HOST"
			if tc.user == customer1 {
				role = "CUSTOMER"
			}
			if _, err := f.service.Floor(context.Background(), auth.User{ID: tc.user, Role: role}, auth.DemoQueueID); !errors.Is(err, auth.ErrForbidden) {
				t.Fatalf("service authorization bypass for %s: %v", tc.user, err)
			}
		}
		if tc.status == 200 {
			if r.body["operationsMode"] != "MANUAL" || len(r.body["zones"].([]any)) != 0 || len(r.body["assignments"].([]any)) != 0 {
				t.Fatalf("empty floor: %v", r.body)
			}
		}
	}
	for _, tc := range []struct {
		id     string
		status int
	}{{"invalid", 404}, {"10000000-0000-4000-8000-000000000099", 403}} {
		r := hostCall(t, f, "GET", hostPath(tc.id, "/floor"), session(t, f, host), false)
		if r.status != tc.status {
			t.Fatalf("path %s: %d", tc.id, r.status)
		}
	}
}
