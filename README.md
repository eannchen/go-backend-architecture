# go-backend-architecture

Go modular-monolith backend architecture template for clean, testable APIs with built-in caching and observability.

## Purpose

- Provide a reusable backend starter that follows clean architecture.
- Keep business logic isolated from frameworks and vendors.
- Offer production-ready foundations: HTTP, DB, cache/KV integration, migrations, tracing, metrics, and structured logging.
- Serve as a base repo to clone for new app projects.

## Architecture and Principles Used

- [Clean Architecture](https://8thlight.com/insights/uncle-bob/2012/08/13/the-clean-architecture.html)
- [Dependency Injection](https://martinfowler.com/articles/injection.html) (constructor-based composition root)
- [SOLID principles](https://en.wikipedia.org/wiki/SOLID)
- [Consumer-owned interfaces](https://go.dev/doc/effective_go#interfaces_and_types)
- [Adapter pattern](https://refactoring.guru/design-patterns/adapter)
- [Decorator pattern](https://refactoring.guru/design-patterns/decorator)
- [Repository pattern](https://martinfowler.com/eaaCatalog/repository.html)
- [Facade pattern](https://refactoring.guru/design-patterns/facade)
- [Builder pattern](https://refactoring.guru/design-patterns/builder)
- [Middleware pattern](https://www.alexedwards.net/blog/making-and-using-middleware)

See inner `README.md` files and `AGENTS.md` for implementation guidance, so AI agents can follow the same architecture rules.

## Third-Party Tools Used

- [`Echo v5`](https://github.com/labstack/echo) for HTTP server
- [`pgx/v5`](https://github.com/jackc/pgx) for PostgreSQL driver and pool
- [`sqlc`](https://github.com/sqlc-dev/sqlc) for static SQL query generation
- [`Masterminds/squirrel`](https://github.com/Masterminds/squirrel) for dynamic SQL building
- [`pressly/goose`](https://github.com/pressly/goose) for DB migrations
- [`go-redis/v9`](https://github.com/redis/go-redis) for Redis client integration
- [`uber-go/zap`](https://github.com/uber-go/zap) for structured logging
- [`air`](https://github.com/air-verse/air) for local hot reload
- [`OpenAPI 3`](https://spec.openapis.org/oas/latest.html) as the shareable HTTP contract format
- [`oapi-codegen`](https://github.com/oapi-codegen/oapi-codegen) for generating backend transport models from the OpenAPI spec
- [`OpenTelemetry`](https://opentelemetry.io/) SDK + [`OTLP`](https://opentelemetry.io/docs/specs/otlp/) exporters for tracing/logs/metrics
- [`HyperDX`](https://www.hyperdx.io/) + [`OpenTelemetry Collector`](https://opentelemetry.io/docs/collector/) for local observability integration
- [`Docker Compose`](https://docs.docker.com/compose/) for local infra orchestration

Why no ORM:

- Raw SQL with [`sqlc`](https://sqlc.dev/) + [`squirrel`](https://github.com/Masterminds/squirrel) gives clear query control, predictable performance tuning, and compile-time type safety.
- Common downsides are handled by:
  - `sqlc` generated typed mappings to reduce runtime schema/query mismatch risk.
  - `squirrel` composable dynamic SQL to avoid fragile string concatenation.
  - clean architecture + repository boundaries to keep SQL isolated in infra adapters and usecases storage-agnostic.

## Use As A Starter

1. Create a new repository from this template.
2. Run:

```bash
./scripts/bootstrap-template.sh --module github.com/your-org/your-backend
```

It updates module/import paths, service/stack naming, OpenAPI title, and README title.

Optional flags:
- `--service-name`: sets the service identity used in `.env.example` and the root README title.
- `--project-slug`: sets the local stack slug used by docker/container/database naming.
- `--api-title`: sets `info.title` in `docs/openapi.yaml`.

3. Run `make openapi-generate && go test ./...`.
4. Review `docker-compose.yml`, `.env.example`, and `docs/openapi.yaml` for project-specific values.
5. Review `AGENTS.md` and inner package `README.md` files before starting feature development.

## Setup and Run

1. `cp .env.example .env`
2. `make install`
3. `make dev-up && make migrate-up`
4. `make run`
5. Check `GET /health?check=ready`

Common commands: `make dev-logs`, `make dev-down`, `make migrate-status`, `make openapi-generate`, `make run-stop`.

Default local ports: Postgres `5432`, Redis `6379`, OTel `4317/4318`, HyperDX `8081`.