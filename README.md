# go-echo-template

Production-shaped Go REST API template: **Echo v5**, **sqlx** (PostgreSQL/pgx), **Infisical** for secrets, **zerolog** for logging, and graceful shutdown wired end to end.

## Stack

| Concern    | Choice                       | Version   |
| ---------- | ---------------------------- | --------- |
| HTTP       | `github.com/labstack/echo/v5`| v5.4.0    |
| Database   | `github.com/jmoiron/sqlx`    | v1.4.0    |
| Driver     | `github.com/jackc/pgx/v5`    | v5.11.0   |
| Secrets    | `github.com/infisical/go-sdk`| v0.8.0    |
| Logging    | `github.com/rs/zerolog`      | v1.35.1   |
| Config     | `github.com/caarlos0/env/v11`| v11.4.1   |
| Validation | `go-playground/validator/v10`| v10.30.5  |
| Go         | toolchain                    | 1.26+     |

> Go 1.26 is required — `validator/v10.30.5` declares `go >= 1.26.0`, and Echo v5 needs ≥ 1.25.

## Layout

```
cmd/api/            entrypoint: signal handling, wiring, self-healthcheck mode
internal/config/    env-backed configuration
internal/secrets/   Infisical loader (injects secrets into the environment)
internal/logger/    zerolog setup + slog bridge for Echo v5
internal/database/  sqlx pool, ping, transaction helper
internal/server/    Echo instance, middleware, routes, error handler, validator
internal/handler/   HTTP handlers
internal/repository/data access
internal/model/     domain types and request DTOs
migrations/         SQL, auto-applied by the postgres container on first boot
```

## Quick start

```bash
cp .env.example .env
docker compose up -d postgres     # or point DB_* at your own instance
make run
```

```bash
curl localhost:8080/health/ready
curl -X POST localhost:8080/api/v1/users \
  -H 'content-type: application/json' \
  -d '{"email":"a@b.com","name":"Ada"}'
```

Everything in one shot: `make up` (api + postgres), `make logs`, `make down`.

## Routes

| Method | Path                 | Notes                              |
| ------ | -------------------- | ---------------------------------- |
| GET    | `/health/live`       | process only — never touches the DB |
| GET    | `/health/ready`      | pings the database                 |
| GET    | `/api/v1/users`      | `?limit=` (≤100) `&offset=`        |
| POST   | `/api/v1/users`      | validated body                     |
| GET    | `/api/v1/users/:id`  | uuid                               |
| PUT    | `/api/v1/users/:id`  | validated body                     |
| DELETE | `/api/v1/users/:id`  | 204                                |

Liveness is deliberately dependency-free so a slow database never causes a restart loop.

## Configuration

All configuration is environment variables — see `.env.example` for the annotated list. `DATABASE_URL` overrides the discrete `DB_*` fields when set. Use `LOG_FORMAT=console` locally and `json` in deployments.

## Infisical

The app uses the **Go SDK**, not the CLI — there is no sidecar, no extra binary in the image, and no init container. At startup `internal/secrets` authenticates with a machine identity (universal auth), lists every secret under the configured path, and exports them as environment variables **before** config is parsed. Any variable in `.env.example` can therefore live in the vault instead.

```bash
INFISICAL_ENABLED=true
INFISICAL_CLIENT_ID=<machine identity client id>
INFISICAL_CLIENT_SECRET=<machine identity client secret>
INFISICAL_PROJECT_ID=<project id>
INFISICAL_ENVIRONMENT=prod
INFISICAL_SECRET_PATH=/
```

Details worth knowing:

- **Disabled by default.** With `INFISICAL_ENABLED=false` the loader is a no-op, so local development and tests run off plain env vars.
- **Local exports win.** A variable already present in the environment is *not* overwritten, unless `INFISICAL_OVERRIDE=true`.
- **Self-hosted** instances: point `INFISICAL_SITE_URL` at your own domain.
- **TLS roots are required** — this is why the scratch image copies `ca-certificates.crt`.
- Only the credential variables go in the Compose file or your orchestrator's secret store; everything else can come from the vault.

## Graceful shutdown

`signal.NotifyContext` catches `SIGINT`/`SIGTERM` and cancels one root context that drives the whole teardown:

1. `echo.StartConfig.Start(ctx, e)` observes the cancellation and stops accepting new connections.
2. In-flight requests get up to `HTTP_SHUTDOWN_TIMEOUT` (default 15s) to finish; `OnShutdownError` logs if they do not.
3. Only after `Run` returns does the deferred `db.Close()` fire, so draining requests keep a working pool.

`ContextTimeout` middleware bounds individual requests (`HTTP_REQUEST_TIMEOUT`) by cancelling the request context, which propagates into sqlx queries. There is a test covering the shutdown path: `TestGracefulShutdown`.

## Docker

The image is `FROM scratch` with a statically linked binary — no shell, no package manager, no libc, and it runs as UID 65532.

```bash
make docker                       # local platform
make docker-multi                 # linux/amd64 + linux/arm64, pushed
```

Multi-platform builds pin the builder to `$BUILDPLATFORM` and cross-compile via `$TARGETOS`/`$TARGETARCH`, so building arm64 on an amd64 host (or the reverse) never runs the compiler under QEMU.

Only three things are added to `scratch`: CA certificates (needed for Infisical and for `sslmode=require`), zoneinfo, and an `/etc/passwd` entry for the non-root user. Because the image has no `curl` or shell, the binary doubles as its own probe — `HEALTHCHECK` runs `/app -healthcheck`.

## Echo v5 notes

v5 is a breaking release; if you are porting v4 code, the traps that matter here:

- Handlers take `*echo.Context` (pointer to a struct), not the `echo.Context` interface.
- `HTTPErrorHandler` is `func(c *echo.Context, err error)` — **arguments swapped** versus v4.
- `echo.ErrNotFound` and friends are no longer `*echo.HTTPError`. A v4-style type assertion silently turns every 404 into a 500, so the error handler uses `echo.StatusCode(err)`. `TestNotFoundUsesJSONErrorHandler` locks this in.
- `middleware.Logger()` and `middleware.Timeout()` are gone — replaced by `RequestLoggerWithConfig` and `ContextTimeout`.
- `middleware.CORSWithConfig` **panics** on an empty `AllowOrigins`, so `registerMiddleware` defaults it.
- `middleware.BodyLimit` takes bytes (`int64`), not a `"2M"` string.
- Echo's own logger is `*slog.Logger`; zerolog's `NewSlogHandler` bridges it so all output lands in one stream.
- `e.Shutdown` no longer exists — server lifecycle lives in `echo.StartConfig`.

## Make targets

```
make run | build | test | lint | tidy | docker | docker-multi | up | down | logs
```

## Renaming the module

```bash
go mod edit -module github.com/you/your-service
grep -rl 'github.com/ysrckr/go-echo-template' --include='*.go' . | xargs sed -i '' 's|github.com/ysrckr/go-echo-template|github.com/you/your-service|g'
```
