package auth

import (
	"encoding/base64"
	"testing"
	"time"
)

func TestTokenAndCSRFAreBoundToSession(t *testing.T) {
	token := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	other := base64.RawURLEncoding.EncodeToString(append([]byte{1}, make([]byte, 31)...))
	if TokenHash(token) == "" || CSRFToken(token) == token || !ValidCSRF(token, CSRFToken(token)) || ValidCSRF(other, CSRFToken(token)) || ValidCSRF("", "") {
		t.Fatal("session/CSRF binding failed")
	}
}

func TestLoginLimiterIsBoundedAndExpires(t *testing.T) {
	now := time.Now()
	limiter := loginLimiter{entries: make(map[string]attempts), now: func() time.Time { return now }}
	for i := 0; i < 10; i++ {
		if !limiter.allow("ip") {
			t.Fatal("attempt denied early")
		}
	}
	if limiter.allow("ip") {
		t.Fatal("brute force not limited")
	}
	now = now.Add(time.Minute)
	if !limiter.allow("ip") {
		t.Fatal("window did not expire")
	}
	for i := 0; i < 10000; i++ {
		limiter.entries[time.Unix(int64(i), 0).String()] = attempts{until: now.Add(time.Minute)}
	}
	if limiter.allow("new-ip") {
		t.Fatal("unbounded limiter storage")
	}
}
