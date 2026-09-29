// Package secrets fetches application secrets from Infisical and injects them
// into the process environment, so the rest of the app only ever reads env vars.
//
// When INFISICAL_ENABLED is false (the default) the loader is a no-op, which
// keeps local development and tests working off a plain .env file.
package secrets

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/caarlos0/env/v11"
	infisical "github.com/infisical/go-sdk"
	"github.com/rs/zerolog"
)

type Config struct {
	Enabled      bool   `env:"INFISICAL_ENABLED"       envDefault:"false"`
	SiteURL      string `env:"INFISICAL_SITE_URL"      envDefault:"https://app.infisical.com"`
	ClientID     string `env:"INFISICAL_CLIENT_ID"`
	ClientSecret string `env:"INFISICAL_CLIENT_SECRET"`
	ProjectID    string `env:"INFISICAL_PROJECT_ID"`
	Environment  string `env:"INFISICAL_ENVIRONMENT"   envDefault:"dev"`
	SecretPath   string `env:"INFISICAL_SECRET_PATH"   envDefault:"/"`
	Recursive    bool   `env:"INFISICAL_RECURSIVE"     envDefault:"true"`
	// Override replaces variables that are already set in the environment.
	// Off by default so an explicit local export always wins.
	Override bool          `env:"INFISICAL_OVERRIDE"      envDefault:"false"`
	Timeout  time.Duration `env:"INFISICAL_TIMEOUT"       envDefault:"15s"`
	// CacheTTL, in seconds, for the SDK's in-memory response cache. 0 disables it.
	CacheTTLSeconds int `env:"INFISICAL_CACHE_TTL_SECONDS" envDefault:"0"`
}

func (c Config) validate() error {
	missing := map[string]string{
		"INFISICAL_CLIENT_ID":     c.ClientID,
		"INFISICAL_CLIENT_SECRET": c.ClientSecret,
		"INFISICAL_PROJECT_ID":    c.ProjectID,
	}
	for name, value := range missing {
		if value == "" {
			return fmt.Errorf("infisical is enabled but %s is not set", name)
		}
	}
	return nil
}

// LoadConfig reads the Infisical settings from the environment.
func LoadConfig() (Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse infisical config: %w", err)
	}
	return cfg, nil
}

// Load authenticates against Infisical with a machine identity (universal
// auth), pulls every secret under the configured path and exports them as
// environment variables.
func Load(ctx context.Context, cfg Config, log zerolog.Logger) error {
	if !cfg.Enabled {
		log.Debug().Msg("infisical disabled, using environment only")
		return nil
	}
	if err := cfg.validate(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	client := infisical.NewInfisicalClient(ctx, infisical.Config{
		SiteUrl:              cfg.SiteURL,
		CacheExpiryInSeconds: cfg.CacheTTLSeconds,
		SilentMode:           true,
	})

	if _, err := client.Auth().UniversalAuthLogin(cfg.ClientID, cfg.ClientSecret); err != nil {
		return fmt.Errorf("infisical login: %w", err)
	}

	result, err := client.Secrets().ListSecrets(infisical.ListSecretsOptions{
		ProjectID:              cfg.ProjectID,
		Environment:            cfg.Environment,
		SecretPath:             cfg.SecretPath,
		Recursive:              cfg.Recursive,
		ExpandSecretReferences: true,
		IncludeImports:         true,
	})
	if err != nil {
		return fmt.Errorf("infisical list secrets: %w", err)
	}

	var injected, skipped int
	for _, secret := range result.Secrets {
		if _, exists := os.LookupEnv(secret.SecretKey); exists && !cfg.Override {
			skipped++
			continue
		}
		if err := os.Setenv(secret.SecretKey, secret.SecretValue); err != nil {
			return fmt.Errorf("set env %s: %w", secret.SecretKey, err)
		}
		injected++
	}

	log.Info().
		Str("environment", cfg.Environment).
		Str("path", cfg.SecretPath).
		Int("injected", injected).
		Int("skipped", skipped).
		Msg("loaded secrets from infisical")

	return nil
}
