// Package logger builds the application's zerolog logger.
package logger

import (
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/rs/zerolog"

	"github.com/ysrckr/go-echo-template/internal/config"
)

// Bootstrap returns a minimal logger for the startup phase, before the full
// configuration (and therefore the real logger) is available.
func Bootstrap() zerolog.Logger {
	return zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).
		Level(zerolog.InfoLevel).
		With().Timestamp().Logger()
}

// New builds the application logger and installs it as zerolog's global logger.
func New(app config.App, cfg config.Log) zerolog.Logger {
	zerolog.TimeFieldFormat = time.RFC3339Nano

	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil || level == zerolog.NoLevel {
		level = zerolog.InfoLevel
	}

	var writer io.Writer = os.Stderr
	if cfg.Format == "console" {
		writer = zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}
	}

	log := zerolog.New(writer).
		Level(level).
		With().
		Timestamp().
		Str("service", app.Name).
		Str("env", app.Environment).
		Str("version", app.Version).
		Logger()

	zerolog.DefaultContextLogger = &log
	return log
}

// Slog adapts a zerolog logger to *slog.Logger, which is what Echo v5 expects
// for its internal logging.
func Slog(log zerolog.Logger) *slog.Logger {
	return slog.New(zerolog.NewSlogHandler(log))
}
