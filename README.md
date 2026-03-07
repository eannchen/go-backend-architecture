# vocynex-api

Backend scaffold for a modular-monolith API using:

- Go + Echo v5
- PostgreSQL (`pgx/v5`)
- `sqlc` for static queries
- Squirrel for dynamic queries
- goose for migrations
- Manual dependency injection (constructor composition)
- Clean Architecture layering

## Quick start

1. Copy env file: `cp .env.example .env`
2. Start local infra: `make dev-up`
3. Run migrations: `make migrate-up`
4. Install dev tools (one-time): `make install`
5. Start server (Air live reload): `make run`
6. Health check: `GET /healthz?check=ready`

## Local development (Docker + Air)

Infra services in `docker-compose.yml`:

- PostgreSQL: `localhost:5432`
- Redis: `localhost:6379`
- OTel Collector: `localhost:4317` (gRPC), `localhost:4318` (HTTP)
- HyperDX UI: `http://localhost:8081`
- HyperDX ClickHouse HTTP: `http://localhost:8123`
- HyperDX local data is persisted under `./volumes/hyperdx/`

Health check modes:

- `GET /healthz?check=live`: process is up (no dependency check).
- `GET /healthz?check=ready`: DB connectivity check (`SELECT 1`).
- `GET /healthz?check=full`: readiness + DB server status query + transactional DB ping.

Recommended flow:

1. `cp .env.example .env`
2. `make dev-up`
3. `make migrate-up`
4. `make install` (one-time)
5. `make run`

Useful commands:

- Tail infra logs: `make dev-logs`
- Stop infra: `make dev-down`
- Migration status: `make migrate-status`
- Start API with Air: `make run`
- Stop stale local API on `:8080`: `make run-stop`

## Migration note (zsh)

If you pass `DB_URL` inline in zsh, quote it because of `?sslmode=...`:

`make migrate-up DB_URL='postgres://postgres:postgres@localhost:5432/vocynex?sslmode=disable'`

Without quoting, zsh may treat `?` as a wildcard.

## Local observability config

- OTel Collector config file is at project root: `otel-collector.yaml`
- Compose mounts it into the collector container at `/etc/otel-collector/config.yaml`
- API OTel environment defaults are in `.env.example`
- `OTEL_EXPORTER_OTLP_ENDPOINT` is the single default endpoint; traces/logs paths are auto-derived (`/v1/traces`, `/v1/logs`).
- Use `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` and `OTEL_EXPORTER_OTLP_LOGS_ENDPOINT` only when custom per-signal endpoints are needed.
- `OTEL_LOG_LEVEL` controls minimum level exported to OTel logs (`LOG_LEVEL` still controls terminal output).
- HTTP requests (including `GET /healthz`) are traced via Echo middleware and exported to OTLP endpoint.

## Layout

- `cmd/api`: process entrypoint and lifecycle.
- `internal/domain`: pure domain models (infra-free).
- `internal/usecase`: application orchestration.
- `internal/repository`: repository interfaces only.
- `internal/service`: domain services.
- `internal/infra`: adapters (config/logger/db/cache/observability).
- `internal/delivery`: HTTP handlers and middleware.
- `pkg`: optional public reusable packages.