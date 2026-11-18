package config

import (
	"errors"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// Config is server-only. Never serialize it or log DatabaseURL.
type Config struct {
	Environment      string
	Port             int
	AppOrigin        string
	DatabaseURL      string
	CloudSQLInstance string
}

func Load(getenv func(string) string) (Config, error) {
	cfg := Config{Environment: getenv("APP_ENV"), AppOrigin: getenv("APP_ORIGIN"), DatabaseURL: getenv("DATABASE_URL"), CloudSQLInstance: getenv("CLOUD_SQL_INSTANCE")}
	if cfg.Environment != "development" && cfg.Environment != "test" && cfg.Environment != "production" {
		return Config{}, errors.New("APP_ENV must be development, test or production")
	}
	// Cloud Run reserves PORT for the ingress container; a non-ingress API
	// sidecar uses API_PORT without competing for the web listener.
	port := getenv("API_PORT")
	if port == "" {
		port = getenv("PORT")
	}
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
	query, err := url.ParseQuery(db.RawQuery)
	if err != nil {
		return Config{}, errors.New("invalid database options")
	}
	if cfg.CloudSQLInstance != "" {
		// Only the explicitly selected managed socket may bypass driver-level TLS.
		// Cloud Run's Cloud SQL integration authenticates/encrypts the remote hop.
		pattern := `^[a-z][a-z0-9-]{4,28}[a-z0-9]:[a-z]+(-[a-z0-9]+)+:[a-z][a-z0-9-]{0,96}$`
		socket := "/cloudsql/" + cfg.CloudSQLInstance
		if cfg.Environment != "production" || !regexp.MustCompile(pattern).MatchString(cfg.CloudSQLInstance) ||
			len(socket+"/.s.PGSQL.5432") >= 108 || db.Host != "localhost" ||
			query.Get("host") != socket || query.Get("sslmode") != "disable" {
			return Config{}, errors.New("invalid managed Cloud SQL socket configuration")
		}
		for key, values := range query {
			if len(values) != 1 || (key != "host" && key != "sslmode" && key != "connect_timeout") {
				return Config{}, errors.New("unsupported managed Cloud SQL database option")
			}
		}
	} else if cfg.Environment == "production" {
		if len(query["sslmode"]) != 1 || query.Get("sslmode") != "verify-full" {
			return Config{}, errors.New("DATABASE_URL requires sslmode=verify-full in production")
		}
		for key, values := range query {
			// Do not let driver options replace the validated endpoint/credentials.
			if len(values) != 1 || strings.Contains(key, "host") || key == "port" || key == "service" ||
				key == "servicefile" || key == "user" || key == "password" || key == "dbname" {
				return Config{}, errors.New("unsupported production database option")
			}
		}
	}
	return cfg, nil
}
