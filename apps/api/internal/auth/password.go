package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

const hashParameters = "m=65536,t=3,p=2"

func HashPassword(password string) (string, error) {
	if len(password) < 12 || len(password) > 128 || !utf8.ValidString(password) {
		return "", errors.New("password must be 12–128 bytes")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", errors.New("cannot generate password salt")
	}
	key := argon2.IDKey([]byte(password), salt, 3, 64*1024, 2, 32)
	return fmt.Sprintf("$argon2id$v=19$%s$%s$%s", hashParameters, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

func VerifyPassword(encoded, password string) bool {
	if len(password) > 128 {
		return false
	}
	parts := strings.Split(encoded, "$")
	// Only the reviewed profile is accepted; never allocate based on unbounded hash parameters.
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" || parts[3] != hashParameters {
		return false
	}
	if len(parts[4]) != 22 || len(parts[5]) != 43 {
		return false
	}
	salt, err := base64.RawStdEncoding.Strict().DecodeString(parts[4])
	if err != nil || len(salt) != 16 {
		return false
	}
	want, err := base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if err != nil || len(want) != 32 {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, 3, 64*1024, 2, 32)
	return subtle.ConstantTimeCompare(got, want) == 1
}
