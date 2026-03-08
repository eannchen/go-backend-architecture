# go-backend-architecture

Go modular-monolith backend architecture template for clean, testable, observability-ready APIs.

## Purpose

- Provide a reusable backend starter that follows clean architecture.
- Keep business logic isolated from frameworks and vendors.
- Offer production-ready foundations: HTTP, DB, migrations, tracing, and structured logging.
- Serve as a base repo to clone for new app projects.

## Architecture and Principles Used

- [Clean Architecture](https://8thlight.com/insights/uncle-bob/2012/08/13/the-clean-architecture.html)
- [Dependency Injection](https://martinfowler.com/articles/injection.html) (constructor-based composition root)
- [SOLID principles](https://en.wikipedia.org/wiki/SOLID)
- [Consumer-owned interfaces](https://go.dev/doc/effective_go#interfaces_and_types)
- [Adapter pattern](https://refactoring.guru/design-patterns/adapter)
- [Repository pattern](https://martinfowler.com/eaaCatalog/repository.html)
- [Facade pattern](https://refactoring.guru/design-patterns/facade)
- [Builder pattern](https://refactoring.guru/design-patterns/builder)
- [Middleware pattern](https://www.alexedwards.net/blog/making-and-using-middleware)

See inner `README.md` files and `.cursor/rules/` for implementation guidance.

## Third-Party Tools Used

- [`Echo v5`](https://github.com/labstack/echo) for HTTP server
- [`pgx/v5`](https://github.com/jackc/pgx) for PostgreSQL driver and pool
- [`sqlc`](https://sqlc.dev/) for static SQL query generation
- [`Masterminds/squirrel`](https://github.com/Masterminds/squirrel) for dynamic SQL building
- [`pressly/goose`](https://github.com/pressly/goose) for DB migrations
- [`air`](https://github.com/air-verse/air) for local hot reload
- [`zap`](https://github.com/uber-go/zap) for structured logging
- [`OpenTelemetry`](https://opentelemetry.io/) SDK + [`OTLP`](https://opentelemetry.io/docs/specs/otlp/) exporters for tracing/logs
- [`HyperDX`](https://www.hyperdx.io/) + [`OpenTelemetry Collector`](https://opentelemetry.io/docs/collector/) for local observability
- [`Docker Compose`](https://docs.docker.com/compose/) for local infra orchestration

Why no ORM:

- Raw SQL with [`sqlc`](https://sqlc.dev/) + [`squirrel`](https://github.com/Masterminds/squirrel) gives clear query control, predictable performance tuning, and compile-time type safety.
- Common downsides are handled by:
  - `sqlc` generated typed mappings to reduce runtime schema/query mismatch risk.
  - `squirrel` composable dynamic SQL to avoid fragile string concatenation.
  - clean architecture + repository boundaries to keep SQL isolated in infra adapters and usecases storage-agnostic.

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