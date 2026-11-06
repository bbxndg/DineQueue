package config

import (
	"strings"
	"testing"
)

func validEnv() map[string]string {
	return map[string]string{"APP_ENV": "development", "APP_ORIGIN": "http://localhost:3000", "DATABASE_URL": "postgres://local:secret@localhost:5432/dinequeue?sslmode=disable", "PORT": "8080"}
}

func TestLoadValidConfig(t *testing.T) {
	env := validEnv()
	cfg, err := Load(func(key string) string { return env[key] })
	if err != nil || cfg.Port != 8080 || cfg.AppOrigin != "http://localhost:3000" {
		t.Fatalf("expected valid development config, got error %v", err)
	}
}

func TestLoadRejectsInvalidConfigWithoutSecrets(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"DATABASE_URL", ""}, {"DATABASE_URL", "postgres://local:secret@/dinequeue"},
		{"DATABASE_URL", "https://local:secret@localhost/dinequeue"},
		{"DATABASE_URL", "postgres://localhost/dinequeue"},
		{"APP_ENV", ""}, {"APP_ENV", "prod"},
		{"PORT", "0"}, {"PORT", "65536"}, {"PORT", "abc"},
		{"APP_ORIGIN", ""}, {"APP_ORIGIN", "http://user:secret@localhost:3000"},
		{"APP_ORIGIN", "http://localhost:3000/path"}, {"APP_ORIGIN", "*"},
	} {
		t.Run(tc.key+"/"+tc.value, func(t *testing.T) {
			env := validEnv()
			env[tc.key] = tc.value
			_, err := Load(func(key string) string { return env[key] })
			if err == nil {
				t.Fatal("invalid configuration accepted")
			}
			if strings.Contains(err.Error(), "secret") {
				t.Fatal("configuration error leaked credentials")
			}
		})
	}
}

func TestProductionRequiresHTTPSAndVerifiedDatabaseTLS(t *testing.T) {
	env := validEnv()
	env["APP_ENV"] = "production"
	if _, err := Load(func(key string) string { return env[key] }); err == nil {
		t.Fatal("unsafe production config accepted")
	}
	env["APP_ORIGIN"] = "https://dinequeue.example.test"
	if _, err := Load(func(key string) string { return env[key] }); err == nil {
		t.Fatal("unverified DB TLS accepted")
	}
	env["DATABASE_URL"] = "postgres://local:secret@db.example.test/dinequeue?sslmode=verify-full"
	if _, err := Load(func(key string) string { return env[key] }); err != nil {
		t.Fatal(err)
	}
}
