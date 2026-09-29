package server_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"

	"github.com/yasar/go-echo-template/internal/config"
	"github.com/yasar/go-echo-template/internal/database"
	"github.com/yasar/go-echo-template/internal/server"
)

// newTestServer builds a server without touching a real database: sqlx.Open is
// lazy, so routes that do not query anything work fine.
func newTestServer(t *testing.T, port int) *server.Server {
	t.Helper()

	db, err := sqlx.Open("pgx", "postgres://invalid:invalid@127.0.0.1:1/none?sslmode=disable")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	cfg := &config.Config{
		App:  config.App{Name: "test", Environment: "test", Version: "test"},
		HTTP: config.HTTP{Host: "127.0.0.1", Port: port, RequestTimeout: 5 * time.Second, ShutdownTimeout: 2 * time.Second, BodyLimitBytes: 1 << 20},
	}

	return server.New(cfg, &database.DB{DB: db}, zerolog.Nop())
}

func TestLivenessProbe(t *testing.T) {
	ts := httptest.NewServer(newTestServer(t, 0).Handler())
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/health/live")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("got status %d, body %s", resp.StatusCode, body)
	}

	var payload map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload["status"] != "ok" {
		t.Fatalf("got status %q, want ok", payload["status"])
	}
}

// TestNotFoundUsesJSONErrorHandler guards the v5 migration trap: echo.ErrNotFound
// is no longer an *echo.HTTPError, so a type assertion in the error handler would
// silently turn every 404 into a 500.
func TestNotFoundUsesJSONErrorHandler(t *testing.T) {
	ts := httptest.NewServer(newTestServer(t, 0).Handler())
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/does-not-exist")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("got status %d, want 404", resp.StatusCode)
	}

	var payload server.ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Message == "" {
		t.Fatal("expected a message in the error body")
	}
}

func TestInvalidUUIDReturnsBadRequest(t *testing.T) {
	ts := httptest.NewServer(newTestServer(t, 0).Handler())
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/api/v1/users/not-a-uuid")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("got status %d, want 400", resp.StatusCode)
	}
}

// TestGracefulShutdown asserts Run returns once the context is cancelled.
func TestGracefulShutdown(t *testing.T) {
	srv := newTestServer(t, 0)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Run(ctx) }()

	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("run returned %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down within 5s")
	}
}
