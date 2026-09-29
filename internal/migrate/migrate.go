// Package migrate embeds the SQL migrations into the binary and applies them
// with goose.
//
// Embedding matters for the deployment model here: the scratch image ships a
// single file, so there is no migrations directory to mount, no goose CLI stage
// and no init container. The same binary that serves traffic owns the schema.
package migrate

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrator applies and inspects the embedded migration set.
type Migrator struct {
	provider *goose.Provider
}

// New builds a Migrator over the embedded migrations.
//
// A Postgres advisory-lock session locker is installed so that rolling deploys
// and multi-replica startups serialise: every replica calls Up, exactly one
// applies, the rest wait and then observe an empty result.
func New(db *sql.DB, log *slog.Logger) (*Migrator, error) {
	// goose globs "*.sql" at the root of the FS, and go:embed keeps the
	// "migrations/" prefix — so hand it the subtree, not the raw embed.FS.
	root, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("sub migrations fs: %w", err)
	}

	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return nil, fmt.Errorf("create migration locker: %w", err)
	}

	provider, err := goose.NewProvider(
		goose.DialectPostgres,
		db,
		root,
		goose.WithSlog(log),
		goose.WithSessionLocker(locker),
	)
	if err != nil {
		return nil, fmt.Errorf("create migration provider: %w", err)
	}

	return &Migrator{provider: provider}, nil
}

// Up applies every pending migration and reports what it did.
func (m *Migrator) Up(ctx context.Context) error {
	results, err := m.provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	for _, r := range results {
		if r.Error != nil {
			return fmt.Errorf("migration %d failed: %w", r.Source.Version, r.Error)
		}
	}
	return nil
}

// Down rolls back the most recently applied migration.
func (m *Migrator) Down(ctx context.Context) error {
	if _, err := m.provider.Down(ctx); err != nil {
		return fmt.Errorf("roll back migration: %w", err)
	}
	return nil
}

// Status returns one line per migration, applied or pending.
func (m *Migrator) Status(ctx context.Context) ([]string, error) {
	statuses, err := m.provider.Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("migration status: %w", err)
	}

	lines := make([]string, 0, len(statuses))
	for _, s := range statuses {
		applied := "pending"
		if !s.AppliedAt.IsZero() {
			applied = s.AppliedAt.Format("2006-01-02 15:04:05")
		}
		lines = append(lines, fmt.Sprintf("%-8d %-12s %s", s.Source.Version, s.State, applied))
	}
	return lines, nil
}

// Version reports the schema version currently recorded in the database.
func (m *Migrator) Version(ctx context.Context) (int64, error) {
	version, err := m.provider.GetDBVersion(ctx)
	if err != nil {
		return 0, fmt.Errorf("migration version: %w", err)
	}
	return version, nil
}

// Pending reports whether unapplied migrations exist, without applying them.
func (m *Migrator) Pending(ctx context.Context) (bool, error) {
	pending, err := m.provider.HasPending(ctx)
	if err != nil {
		return false, fmt.Errorf("check pending migrations: %w", err)
	}
	return pending, nil
}
