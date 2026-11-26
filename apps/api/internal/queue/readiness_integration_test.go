//go:build integration

package queue

import (
	"dinequeue.local/api/internal/auth"
	"testing"
)

func TestL7NewJoinStartsNotReady(t *testing.T) {
	f := setup(t)
	r := joinNotReady(t, f)
	ready, exists := r.body["isReady"]
	if !exists || ready != false {
		t.Fatalf("new ticket must explicitly start Not ready; got %v", r.body)
	}
}

func TestL7NoReadyPartiesCannotBeCalled(t *testing.T) {
	f := setup(t)
	joinNotReady(t, f)
	r := hostCall(t, f, "POST", hostPath(auth.DemoQueueID, "/call-next"), session(t, f, host), true)
	if r.status != 409 || errorCode(r) != "NO_READY_PARTIES" {
		t.Fatalf("unconfirmed party must not be called: HTTP %d %v", r.status, r.body)
	}
}

func joinNotReady(t *testing.T, f fixture) reply {
	t.Helper()
	r := join(t, f, auth.DemoQueueID, `{"partySize":2}`, session(t, f, newUser(t, f, "CUSTOMER")), true)
	if r.status != 201 {
		t.Fatalf("join %d %v", r.status, r.body)
	}
	return r
}
