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

func TestProductionCloudSQLSocketIsExplicitAndBoundToInstance(t *testing.T) {
	base := validEnv()
	base["APP_ENV"] = "production"
	base["APP_ORIGIN"] = "https://dinequeue.example.test"
	base["CLOUD_SQL_INSTANCE"] = "dinequeue-test:asia-southeast1:dinequeue-pg"
	base["DATABASE_URL"] = "postgres://app:secret@localhost/dinequeue_demo?host=/cloudsql/dinequeue-test:asia-southeast1:dinequeue-pg&sslmode=disable"
	if _, err := Load(func(key string) string { return base[key] }); err != nil {
		t.Fatal("explicit managed Cloud SQL socket rejected")
	}
	for _, tc := range []struct{ key, value string }{
		{"CLOUD_SQL_INSTANCE", ""},
		{"CLOUD_SQL_INSTANCE", "../other"},
		{"APP_ENV", "development"},
		{"DATABASE_URL", "postgres://app:secret@localhost/dinequeue_demo?sslmode=disable"},
		{"DATABASE_URL", "postgres://app:secret@localhost/dinequeue_demo?host=/tmp/other&sslmode=disable"},
		{"DATABASE_URL", "postgres://app:secret@localhost/dinequeue_demo?host=/cloudsql/dinequeue-test:asia-southeast1:other&sslmode=disable"},
		{"DATABASE_URL", "postgres://app:secret@localhost/dinequeue_demo?host=/cloudsql/dinequeue-test:asia-southeast1:dinequeue-pg&host=evil&sslmode=disable"},
		{"DATABASE_URL", "postgres://app:secret@localhost/dinequeue_demo?host=/cloudsql/dinequeue-test:asia-southeast1:dinequeue-pg&sslmode=disable&sslmode=verify-full"},
		{"DATABASE_URL", "postgres://app:secret@evil/dinequeue_demo?host=/cloudsql/dinequeue-test:asia-southeast1:dinequeue-pg&sslmode=disable"},
		{"DATABASE_URL", "postgres://app:secret@localhost/dinequeue_demo?host=/cloudsql/dinequeue-test:asia-southeast1:dinequeue-pg&sslmode=disable&hostaddr=8.8.8.8"},
	} {
		env := make(map[string]string)
		for k, v := range base {
			env[k] = v
		}
		env[tc.key] = tc.value
		if _, err := Load(func(key string) string { return env[key] }); err == nil {
			t.Fatalf("invalid Cloud SQL transport accepted for %s", tc.key)
		}
	}
}

func TestProductionTCPRejectsTransportOverrides(t *testing.T) {
	for _, query := range []string{
		"sslmode=verify-full&sslmode=disable", "sslmode=verify-full&host=localhost",
		"sslmode=verify-full&hostaddr=127.0.0.1", "sslmode=verify-full&service=other",
	} {
		env := validEnv()
		env["APP_ENV"] = "production"
		env["APP_ORIGIN"] = "https://dinequeue.example.test"
		env["DATABASE_URL"] = "postgres://app:secret@db.example.test/dinequeue?" + query
		if _, err := Load(func(key string) string { return env[key] }); err == nil {
			t.Fatal("production transport override accepted")
		}
	}
}

func TestSidecarAPIPortOverridesIngressPort(t *testing.T) {
	env := validEnv()
	env["PORT"] = "8080"
	env["API_PORT"] = "8081"
	cfg, err := Load(func(key string) string { return env[key] })
	if err != nil || cfg.Port != 8081 {
		t.Fatal("sidecar listener did not use API_PORT")
	}
	env["API_PORT"] = "invalid"
	if _, err := Load(func(key string) string { return env[key] }); err == nil {
		t.Fatal("invalid API_PORT accepted")
	}
}
