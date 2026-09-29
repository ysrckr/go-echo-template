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
	"github.com/ysrckr/go-echo-template/internal/secrets"
	"github.com/ysrckr/go-echo-template/internal/server"
)

func main() {
	healthcheck := flag.Bool("healthcheck", false, "probe the local readiness endpoint and exit (used as the container HEALTHCHECK)")
	flag.Parse()

	if *healthcheck {
		os.Exit(probe())
	}

	if err := run(); err != nil {
		boot := logger.Bootstrap()
		boot.Fatal().Err(err).Msg("application stopped")
	}
}

func run() error {
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

	srv := server.New(cfg, db, log)

	if err := srv.Run(ctx); err != nil {
		return err
	}

	log.Info().Msg("shutdown complete")
	return nil
}
