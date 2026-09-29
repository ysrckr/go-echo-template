// Package config loads application configuration from environment variables.
//
// Secrets are expected to already be present in the environment by the time
// Load is called — see internal/secrets, which pulls them from Infisical and
// injects them before this package runs.
package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	App      App
	HTTP     HTTP
	Database Database
	Log      Log
}

type App struct {
	Name        string `env:"APP_NAME"        envDefault:"go-echo-template"`
	Environment string `env:"APP_ENV"         envDefault:"development"`
	Version     string `env:"APP_VERSION"     envDefault:"dev"`
}

func (a App) IsProduction() bool { return a.Environment == "production" }

type HTTP struct {
	Host string `env:"HTTP_HOST" envDefault:"0.0.0.0"`
	Port int    `env:"HTTP_PORT" envDefault:"8080"`

	ReadTimeout     time.Duration `env:"HTTP_READ_TIMEOUT"     envDefault:"10s"`
	WriteTimeout    time.Duration `env:"HTTP_WRITE_TIMEOUT"    envDefault:"20s"`
	IdleTimeout     time.Duration `env:"HTTP_IDLE_TIMEOUT"     envDefault:"120s"`
	RequestTimeout  time.Duration `env:"HTTP_REQUEST_TIMEOUT"  envDefault:"30s"`
	ShutdownTimeout time.Duration `env:"HTTP_SHUTDOWN_TIMEOUT" envDefault:"15s"`

	// v5's BodyLimit middleware takes a byte count rather than a "2M" string.
	BodyLimitBytes int64    `env:"HTTP_BODY_LIMIT_BYTES" envDefault:"2097152"`
	AllowedOrigins []string `env:"HTTP_ALLOWED_ORIGINS" envDefault:"*" envSeparator:","`
}

func (h HTTP) Addr() string { return fmt.Sprintf("%s:%d", h.Host, h.Port) }

type Database struct {
	// DSN wins when set; otherwise it is assembled from the discrete fields.
	DSN      string `env:"DATABASE_URL"`
	Host     string `env:"DB_HOST"     envDefault:"localhost"`
	Port     int    `env:"DB_PORT"     envDefault:"5432"`
	User     string `env:"DB_USER"     envDefault:"postgres"`
	Password string `env:"DB_PASSWORD"`
	Name     string `env:"DB_NAME"     envDefault:"postgres"`
	SSLMode  string `env:"DB_SSLMODE"  envDefault:"disable"`

	MaxOpenConns    int           `env:"DB_MAX_OPEN_CONNS"     envDefault:"25"`
	MaxIdleConns    int           `env:"DB_MAX_IDLE_CONNS"     envDefault:"25"`
	ConnMaxLifetime time.Duration `env:"DB_CONN_MAX_LIFETIME"  envDefault:"5m"`
	ConnMaxIdleTime time.Duration `env:"DB_CONN_MAX_IDLE_TIME" envDefault:"5m"`
	ConnectTimeout  time.Duration `env:"DB_CONNECT_TIMEOUT"    envDefault:"10s"`

	// AutoMigrate applies pending migrations during startup, before the server
	// accepts traffic. Turn it off to run migrations as a separate deploy step
	// (`app -migrate up`).
	AutoMigrate    bool          `env:"DB_AUTO_MIGRATE"   envDefault:"true"`
	MigrateTimeout time.Duration `env:"DB_MIGRATE_TIMEOUT" envDefault:"60s"`
}

func (d Database) ConnString() string {
	if d.DSN != "" {
		return d.DSN
	}
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		d.User, d.Password, d.Host, d.Port, d.Name, d.SSLMode,
	)
}

type Log struct {
	Level string `env:"LOG_LEVEL"  envDefault:"info"`
	// "json" for machine-readable output, "console" for human-readable.
	Format string `env:"LOG_FORMAT" envDefault:"console"`
}

// Load parses the process environment into a Config.
func Load() (*Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}
