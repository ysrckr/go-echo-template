// Package server wires the Echo (v5) HTTP server: middleware, routes and a
// context-driven graceful shutdown.
package server

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/rs/zerolog"

	"github.com/ysrckr/go-echo-template/internal/config"
	"github.com/ysrckr/go-echo-template/internal/database"
	"github.com/ysrckr/go-echo-template/internal/handler"
	"github.com/ysrckr/go-echo-template/internal/logger"
	"github.com/ysrckr/go-echo-template/internal/repository"
)

type Server struct {
	echo *echo.Echo
	cfg  *config.Config
	log  zerolog.Logger
}

func New(cfg *config.Config, db *database.DB, log zerolog.Logger) *Server {
	// v5 configures the instance up front instead of mutating fields after New().
	e := echo.NewWithConfig(echo.Config{
		Logger:           logger.Slog(log),
		Validator:        NewValidator(),
		HTTPErrorHandler: errorHandler(log, !cfg.App.IsProduction()),
		IPExtractor:      echo.ExtractIPFromXFFHeader(),
	})

	registerMiddleware(e, cfg, log)

	userRepo := repository.NewUserRepository(db)
	registerRoutes(e, handler.NewHealth(db, cfg.App), handler.NewUser(userRepo))

	return &Server{echo: e, cfg: cfg, log: log}
}

func registerMiddleware(e *echo.Echo, cfg *config.Config, log zerolog.Logger) {
	e.Use(middleware.RequestID())
	e.Use(middleware.Recover())
	e.Use(middleware.Gzip())
	e.Use(middleware.BodyLimit(cfg.HTTP.BodyLimitBytes))

	// v5 replaced middleware.Timeout with ContextTimeout, which cancels the
	// request context rather than racing the handler goroutine.
	e.Use(middleware.ContextTimeout(cfg.HTTP.RequestTimeout))

	// v5 rejects an empty AllowOrigins outright (it panics at construction), so
	// never hand it an unset slice. Note it also refuses "*" together with
	// AllowCredentials — list explicit origins if you need credentialed CORS.
	allowedOrigins := cfg.HTTP.AllowedOrigins
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{"*"}
	}

	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: allowedOrigins,
		AllowMethods: []string{
			http.MethodGet, http.MethodHead, http.MethodPost,
			http.MethodPut, http.MethodPatch, http.MethodDelete,
		},
		AllowHeaders: []string{
			echo.HeaderOrigin, echo.HeaderContentType,
			echo.HeaderAccept, echo.HeaderAuthorization,
		},
		MaxAge: 300,
	}))

	e.Use(requestLogger(log))
}

// requestLogger feeds Echo's RequestLogger values into zerolog. v5 removed
// middleware.Logger(), so this is the supported way to log requests.
func requestLogger(log zerolog.Logger) echo.MiddlewareFunc {
	return middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogLatency:   true,
		LogRemoteIP:  true,
		LogMethod:    true,
		LogURIPath:   true,
		LogRoutePath: true,
		LogStatus:    true,
		LogRequestID: true,
		LogUserAgent: true,
		// Let our HTTPErrorHandler own the response; we only record the error.
		HandleError: false,
		Skipper: func(c *echo.Context) bool {
			return strings.HasPrefix(c.Request().URL.Path, "/health")
		},
		LogValuesFunc: func(c *echo.Context, v middleware.RequestLoggerValues) error {
			event := log.Info()
			switch {
			case v.Error != nil || v.Status >= http.StatusInternalServerError:
				event = log.Error().Err(v.Error)
			case v.Status >= http.StatusBadRequest:
				event = log.Warn().Err(v.Error)
			}

			event.
				Str("request_id", v.RequestID).
				Str("method", v.Method).
				Str("path", v.URIPath).
				Str("route", v.RoutePath).
				Int("status", v.Status).
				Dur("latency", v.Latency).
				Str("remote_ip", v.RemoteIP).
				Str("user_agent", v.UserAgent).
				Msg("request")

			return nil
		},
	})
}

// Run serves until ctx is cancelled, then drains in-flight requests.
//
// echo.StartConfig.Start blocks on ctx and performs the graceful shutdown
// itself, waiting up to GracefulTimeout for open requests to finish.
func (s *Server) Run(ctx context.Context) error {
	start := echo.StartConfig{
		Address:         s.cfg.HTTP.Addr(),
		HideBanner:      true,
		HidePort:        true,
		GracefulTimeout: s.cfg.HTTP.ShutdownTimeout,
		BeforeServeFunc: func(srv *http.Server) error {
			srv.ReadTimeout = s.cfg.HTTP.ReadTimeout
			srv.ReadHeaderTimeout = s.cfg.HTTP.ReadTimeout
			srv.WriteTimeout = s.cfg.HTTP.WriteTimeout
			srv.IdleTimeout = s.cfg.HTTP.IdleTimeout
			s.log.Info().Str("addr", s.cfg.HTTP.Addr()).Msg("http server listening")
			return nil
		},
		OnShutdownError: func(err error) {
			s.log.Error().Err(err).Msg("graceful shutdown did not finish in time")
		},
	}

	if err := start.Start(ctx, s.echo); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	s.log.Info().Msg("http server stopped")
	return nil
}

// Handler exposes the Echo instance for tests (echo.Echo implements http.Handler).
func (s *Server) Handler() http.Handler { return s.echo }
