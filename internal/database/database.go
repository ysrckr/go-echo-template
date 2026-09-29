// Package database opens and manages the sqlx connection pool.
package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"

	// Registers the "pgx" driver used below.
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/ysrckr/go-echo-template/internal/config"
)

// DB wraps *sqlx.DB so the rest of the app depends on our type, not the driver.
type DB struct {
	*sqlx.DB
}

// New opens the pool and verifies connectivity with a ping.
func New(ctx context.Context, cfg config.Database, log zerolog.Logger) (*DB, error) {
	db, err := sqlx.Open("pgx", cfg.ConnString())
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	pingCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	log.Info().
		Int("max_open_conns", cfg.MaxOpenConns).
		Int("max_idle_conns", cfg.MaxIdleConns).
		Msg("database connected")

	return &DB{DB: db}, nil
}

// SQL exposes the underlying *sql.DB, which is what goose expects.
func (d *DB) SQL() *sql.DB {
	return d.DB.DB
}

// Health reports whether the pool can still reach the database.
func (d *DB) Health(ctx context.Context) error {
	return d.PingContext(ctx)
}

// WithTx runs fn inside a transaction, committing on success and rolling back
// on error or panic.
func (d *DB) WithTx(ctx context.Context, fn func(*sqlx.Tx) error) (err error) {
	tx, err := d.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback()
			return
		}
		if cErr := tx.Commit(); cErr != nil {
			err = fmt.Errorf("commit tx: %w", cErr)
		}
	}()

	return fn(tx)
}
