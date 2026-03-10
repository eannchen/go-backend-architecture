# go-backend-architecture
[![Go Report Card](https://goreportcard.com/badge/github.com/eannchen/go-backend-architecture)](https://goreportcard.com/report/github.com/eannchen/go-backend-architecture)

Go modular-monolith backend template for building maintainable, testable APIs with built-in caching and observability.

## Purpose

- Provide a reusable backend starter that follows clean architecture.
- Keep business logic isolated from frameworks and vendors.
- Offer production-ready foundations: HTTP, DB, cache/KV integration, migrations, tracing, metrics, and structured logging.
- Serve as a base repo to clone for new app projects.

## Architecture and Principles

- [Clean Architecture](https://8thlight.com/insights/uncle-bob/2012/08/13/the-clean-architecture.html)
- [Dependency Injection](https://martinfowler.com/articles/injection.html)
- [SOLID principles](https://en.wikipedia.org/wiki/SOLID)
- [Consumer-owned interfaces](https://go.dev/doc/effective_go#interfaces_and_types)
- [Adapter pattern](https://refactoring.guru/design-patterns/adapter)
- [Decorator pattern](https://refactoring.guru/design-patterns/decorator)
- [Facade pattern](https://refactoring.guru/design-patterns/facade)
- [Builder pattern](https://refactoring.guru/design-patterns/builder)
- [Repository pattern](https://martinfowler.com/eaaCatalog/repository.html)
- [Middleware pattern](https://www.alexedwards.net/blog/making-and-using-middleware)

See package-level `README.md` files and `AGENTS.md` for implementation guidance and shared architecture rules for both engineers and AI agents.

## Third-Party Tools

- [`Echo v5`](https://github.com/labstack/echo) - HTTP server
- [`pgx/v5`](https://github.com/jackc/pgx) - PostgreSQL driver and connection pool
- [`sqlc`](https://github.com/sqlc-dev/sqlc) - static SQL query generation
- [`Masterminds/squirrel`](https://github.com/Masterminds/squirrel) - dynamic SQL construction
- [`pressly/goose`](https://github.com/pressly/goose) - database migrations
- [`go-redis/v9`](https://github.com/redis/go-redis) - Redis client integration
- [`uber-go/zap`](https://github.com/uber-go/zap) - structured logging
- [`air`](https://github.com/air-verse/air) - local hot reload
- [`OpenAPI 3`](https://spec.openapis.org/oas/latest.html) - source of truth for HTTP contracts
- [`oapi-codegen`](https://github.com/oapi-codegen/oapi-codegen) - backend transport model generation from OpenAPI
- [`OpenTelemetry`](https://opentelemetry.io/) SDK + [`OTLP`](https://opentelemetry.io/docs/specs/otlp/) exporters - tracing, logs, and metrics
- [`HyperDX`](https://www.hyperdx.io/) + [`OpenTelemetry Collector`](https://opentelemetry.io/docs/collector/) - local observability integration
- [`Docker Compose`](https://docs.docker.com/compose/) - local infrastructure orchestration

Why SQL-first data access (no ORM):

- Raw SQL with [`sqlc`](https://sqlc.dev/) + [`squirrel`](https://github.com/Masterminds/squirrel) provides explicit query control, predictable performance tuning, and compile-time type safety.
- Common downsides are handled by:
  - `sqlc` generated typed mappings to reduce runtime schema/query mismatch risk.
  - `squirrel` composable dynamic SQL to avoid fragile string concatenation.
  - Clean Architecture + repository boundaries to isolate SQL in infra adapters and keep usecases storage-agnostic.

## Requirements

- Go (current stable version)
- Docker + Docker Compose
- GNU Make

## Use as a Starter

1. Create a new repository from this template.
2. Bootstrap the project:

```bash
./scripts/bootstrap-template.sh --module github.com/your-org/your-backend
```

This updates module/import paths, service and stack naming, OpenAPI title, and README title.

Optional flags:
- `--service-name`: sets the service identity used in `.env.example` and the root README title.
- `--project-slug`: sets the local stack slug used by docker/container/database naming.
- `--api-title`: sets `info.title` in `docs/openapi.yaml`.

3. If you cloned this repository directly, rename your local project directory and update the Git remote URL to your new repository. (`bootstrap-template.sh` does not change directory names or Git remotes.)
4. Validate with `make openapi-generate && go test ./...`.
5. Review `docker-compose.yml`, `.env.example`, and `docs/openapi.yaml` for project-specific values. The included account/health code is example domain; replace or remove it and add your own migrations and features.
6. Review `AGENTS.md` and package-level `README.md` files before feature development.

## Setup and Run

1. `cp .env.example .env`
2. `make install`
3. `make dev-up && make migrate-up`
4. `make run`
5. Verify `GET /health?check=ready`

Common commands: `make dev-logs`, `make dev-down`, `make migrate-status`, `make openapi-generate`, `make run-stop`.

Default local ports: Postgres `5432`, Redis `6379`, OTel `4317/4318`, HyperDX `8081`.