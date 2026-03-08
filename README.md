# go-backend-architecture

Go backend architecture template for building modular, maintainable APIs quickly.

## Purpose

- Provide a reusable backend starter that follows clean architecture.
- Keep business logic isolated from frameworks and vendors.
- Offer production-ready foundations: HTTP, DB, migrations, tracing, and structured logging.
- Serve as a base repo to clone for new app projects.

## Setup and Run

1. Copy environment file: `cp .env.example .env`
2. Install tools (one-time): `make install`
3. Start local infra: `make dev-up`
4. Run migrations: `make migrate-up`
5. Start API with live reload: `make run`
6. Check health endpoint: `GET /health?check=ready`

Useful commands:

- Tail infra logs: `make dev-logs`
- Stop infra: `make dev-down`
- Check migration status: `make migrate-status`
- Stop stale API process on `:8080`: `make run-stop`

Notes:

- For zsh inline migration URL, quote `DB_URL`:
  - `make migrate-up DB_URL='postgres://postgres:postgres@localhost:5432/vocynex?sslmode=disable'`
- Local infra endpoints:
  - PostgreSQL: `localhost:5432`
  - Redis: `localhost:6379`
  - OTel Collector: `localhost:4317` (gRPC), `localhost:4318` (HTTP)
  - HyperDX UI: `http://localhost:8081`

## Architecture and Principles Used

- Clean Architecture
- Dependency Injection (constructor-based composition root)
- SOLID principles
- Consumer-owned interfaces
- Adapter pattern
- Repository pattern
- Facade pattern
- Builder pattern
- Middleware pattern

See inner `README.md` files and `.cursor/rules/` for implementation guidance.

## Third-Party Tools Used

- `echo/v5` for HTTP server
- `pgx/v5` for PostgreSQL driver and pool
- `sqlc` for static SQL query generation
- `Masterminds/squirrel` for dynamic SQL building
- `pressly/goose` for DB migrations
- `air` for local hot reload
- `zap` for structured logging
- OpenTelemetry SDK + OTLP exporters for tracing/logs
- Docker Compose for local infra orchestration
- HyperDX + OTel Collector for local observability