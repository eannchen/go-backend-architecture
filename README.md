# vocynex-api

Backend scaffold for a modular-monolith API using:

- Go + Echo
- PostgreSQL (`pgx/v5`)
- `sqlc` for static queries
- Squirrel for dynamic queries
- goose for migrations
- Manual dependency injection (constructor composition)
- Clean Architecture layering

## Quick start

1. Copy env file: `cp .env.example .env`
2. Run migrations: `make migrate-up DB_URL=<postgres-url>`
3. Start server: `make run`
4. Health check: `GET /healthz`

## Layout

- `cmd/api`: process entrypoint and lifecycle.
- `internal/domain`: pure domain models (infra-free).
- `internal/usecase`: application orchestration.
- `internal/repository`: repository interfaces only.
- `internal/service`: domain services.
- `internal/infra`: adapters (config/logger/db/cache/observability).
- `internal/delivery`: HTTP handlers and middleware.
- `pkg`: optional public reusable packages.