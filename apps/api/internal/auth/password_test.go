package auth

import (
	"strings"
	"testing"
)

func TestPasswordHashSaltAndVerification(t *testing.T) {
	password := "test-only-password-123"
	first, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	second, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if first == second || strings.Contains(first, password) {
		t.Fatal("password hashes must have distinct salts and hide plaintext")
	}
	if !VerifyPassword(first, password) || VerifyPassword(first, "wrong") {
		t.Fatal("password verification failed")
	}
	for _, bad := range []string{"", "$argon2id$v=19$m=999999999,t=3,p=2$a$b", strings.Replace(first, "v=19", "v=18", 1)} {
		if VerifyPassword(bad, password) {
			t.Fatal("malformed/unsupported hash accepted")
		}
	}
	if _, err := HashPassword("short"); err == nil {
		t.Fatal("short seed password accepted")
	}
	if _, err := HashPassword(strings.Repeat("x", 129)); err == nil {
		t.Fatal("oversized password accepted")
	}
}
