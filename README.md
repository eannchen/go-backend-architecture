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
6. Health check: `GET /healthz`

## Local development (Docker + Air)

Infra services in `docker-compose.yml`:

- PostgreSQL: `localhost:5432`
- Redis: `localhost:6379`
- OTel Collector: `localhost:4317` (gRPC), `localhost:4318` (HTTP)
- HyperDX UI: `http://localhost:8081`
- HyperDX ClickHouse HTTP: `http://localhost:8123`

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

## Migration note (zsh)

If you pass `DB_URL` inline in zsh, quote it because of `?sslmode=...`:

`make migrate-up DB_URL='postgres://postgres:postgres@localhost:5432/vocynex?sslmode=disable'`

Without quoting, zsh may treat `?` as a wildcard.

## Local observability config

- OTel Collector config file is at project root: `otel-collector.yaml`
- Compose mounts it into the collector container at `/etc/otel-collector/config.yaml`
- API OTel environment defaults are in `.env.example`

## Layout

- `cmd/api`: process entrypoint and lifecycle.
- `internal/domain`: pure domain models (infra-free).
- `internal/usecase`: application orchestration.
- `internal/repository`: repository interfaces only.
- `internal/service`: domain services.
- `internal/infra`: adapters (config/logger/db/cache/observability).
- `internal/delivery`: HTTP handlers and middleware.
- `pkg`: optional public reusable packages.