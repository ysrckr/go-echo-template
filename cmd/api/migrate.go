package main

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/ysrckr/go-echo-template/internal/migrate"
)

// runMigration executes a one-shot `-migrate <command>` and returns.
func runMigration(ctx context.Context, m *migrate.Migrator, command string, timeout time.Duration, log zerolog.Logger) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	switch command {
	case "up":
		if err := m.Up(ctx); err != nil {
			return err
		}
		version, err := m.Version(ctx)
		if err != nil {
			return err
		}
		log.Info().Int64("schema_version", version).Msg("migrations applied")

	case "down":
		if err := m.Down(ctx); err != nil {
			return err
		}
		version, err := m.Version(ctx)
		if err != nil {
			return err
		}
		log.Info().Int64("schema_version", version).Msg("migration rolled back")

	case "status":
		lines, err := m.Status(ctx)
		if err != nil {
			return err
		}
		fmt.Printf("%-8s %-12s %s\n", "VERSION", "STATE", "APPLIED AT")
		for _, line := range lines {
			fmt.Println(line)
		}

	case "version":
		version, err := m.Version(ctx)
		if err != nil {
			return err
		}
		fmt.Println(version)

	default:
		return fmt.Errorf("unknown -migrate command %q (want: up, down, status, version)", command)
	}

	return nil
}
