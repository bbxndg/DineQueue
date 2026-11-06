package config

import (
	"errors"
	"net/url"
	"strconv"
)

// Config is server-only. Never serialize it or log DatabaseURL.
type Config struct {
	Environment string
	Port        int
	AppOrigin   string
	DatabaseURL string
}

func Load(getenv func(string) string) (Config, error) {
	cfg := Config{Environment: getenv("APP_ENV"), AppOrigin: getenv("APP_ORIGIN"), DatabaseURL: getenv("DATABASE_URL")}
	if cfg.Environment != "development" && cfg.Environment != "test" && cfg.Environment != "production" {
		return Config{}, errors.New("APP_ENV must be development, test or production")
	}
	port := getenv("PORT")
	if port == "" {
		port = "8080"
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return Config{}, errors.New("PORT must be between 1 and 65535")
	}
	cfg.Port = n
	origin, err := url.Parse(cfg.AppOrigin)
	if err != nil || origin.Hostname() == "" || origin.User != nil || origin.RawQuery != "" || origin.Fragment != "" ||
		(origin.Path != "" && origin.Path != "/") || (origin.Scheme != "http" && origin.Scheme != "https") {
		return Config{}, errors.New("APP_ORIGIN must be an HTTP(S) origin")
	}
	if cfg.Environment == "production" && origin.Scheme != "https" {
		return Config{}, errors.New("APP_ORIGIN requires HTTPS in production")
	}
	db, err := url.Parse(cfg.DatabaseURL)
	if err != nil || db.Hostname() == "" || db.User == nil || db.User.Username() == "" || db.Path == "" || db.Path == "/" ||
		(db.Scheme != "postgres" && db.Scheme != "postgresql") || db.Fragment != "" {
		return Config{}, errors.New("DATABASE_URL must be a PostgreSQL URL with host, database and credentials")
	}
	if password, ok := db.User.Password(); !ok || password == "" {
		return Config{}, errors.New("DATABASE_URL requires a password")
	}
	if cfg.Environment == "production" && db.Query().Get("sslmode") != "verify-full" {
		return Config{}, errors.New("DATABASE_URL requires sslmode=verify-full in production")
	}
	return cfg, nil
}
