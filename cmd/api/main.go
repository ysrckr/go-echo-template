package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"

	"github.com/ysrckr/go-echo-template/internal/config"
	"github.com/ysrckr/go-echo-template/internal/database"
	"github.com/ysrckr/go-echo-template/internal/logger"
	"github.com/ysrckr/go-echo-template/internal/migrate"
	"github.com/ysrckr/go-echo-template/internal/secrets"
	"github.com/ysrckr/go-echo-template/internal/server"
)

func main() {
	healthcheck := flag.Bool("healthcheck", false, "probe the local readiness endpoint and exit (used as the container HEALTHCHECK)")
	migrateCmd := flag.String("migrate", "", "run migrations and exit: up | down | status | version")
	flag.Parse()

	if *healthcheck {
		os.Exit(probe())
	}

	if err := run(*migrateCmd); err != nil {
		boot := logger.Bootstrap()
		boot.Fatal().Err(err).Msg("application stopped")
	}
}

func run(migrateCmd string) error {
	// One context for the whole process: SIGINT/SIGTERM cancels it, which in
	// turn drives the HTTP server's graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	boot := logger.Bootstrap()

	// .env is a local convenience; in deployed environments Infisical is the
	// source of truth and a missing file is not an error.
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		boot.Debug().Err(err).Msg("no .env file loaded")
	}

	infisicalCfg, err := secrets.LoadConfig()
	if err != nil {
		return err
	}
	if err := secrets.Load(ctx, infisicalCfg, boot); err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	log := logger.New(cfg.App, cfg.Log)
	log.Info().Msg("starting service")

	db, err := database.New(ctx, cfg.Database, log)
	if err != nil {
		return err
	}
	// Closed only after Run returns, so in-flight requests keep their pool.
	defer func() {
		if err := db.Close(); err != nil {
			log.Error().Err(err).Msg("closing database")
		} else {
			log.Info().Msg("database connection closed")
		}
	}()

	migrator, err := migrate.New(db.SQL(), logger.Slog(log))
	if err != nil {
		return err
	}

	// `-migrate <cmd>` is a one-shot admin mode: run it and exit without
	// starting the server. Useful as a pre-deploy job or a Kubernetes initContainer.
	if migrateCmd != "" {
		return runMigration(ctx, migrator, migrateCmd, cfg.Database.MigrateTimeout, log)
	}

	if cfg.Database.AutoMigrate {
		migrateCtx, cancel := context.WithTimeout(ctx, cfg.Database.MigrateTimeout)
		defer cancel()

		// An advisory lock inside goose serialises this across replicas.
		if err := migrator.Up(migrateCtx); err != nil {
			return err
		}

		version, err := migrator.Version(migrateCtx)
		if err != nil {
			return err
		}
		log.Info().Int64("schema_version", version).Msg("migrations up to date")
	}

	srv := server.New(cfg, db, log)

	if err := srv.Run(ctx); err != nil {
		return err
	}

	log.Info().Msg("shutdown complete")
	return nil
}
