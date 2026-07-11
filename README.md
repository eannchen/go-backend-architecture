# Go Backend Architecture

![Go Version](https://img.shields.io/github/go-mod/go-version/eannchen/go-backend-architecture)

Go modular-monolith backend template with Clean Architecture — clear layer boundaries, production-ready API foundations, and agent rules that keep AI-assisted changes consistent.

## Included Foundations

- **Multi-binary-ready composition** — support for adding future workers as sibling process packages without duplicating runtime setup.
- **HTTP safety and streaming** — timeouts, graceful shutdown, CORS allowlists, trusted-proxy IP extraction, security headers, and reusable Server-Sent Events with a bounded health-stream demo.
- **Authentication** — Redis-backed sessions and OTPs, optional Resend email delivery, optional Google OAuth, and secure cookie defaults.
- **Rate limiting** — app-level Redis token-bucket per-IP limiting at the origin, useful without a CDN and as defense in depth behind one.
- **Data stores** — SQL-first PostgreSQL, Redis cache-aside store composition, and a ready-to-wire object-storage adapter — with vendor types kept inside infra.
- **Observability** — OpenTelemetry tracing and metrics, plus structured logging — kept behind app-owned interfaces.
- **Testing** — reusable repository fakes for unit tests, plus HTTP integration suites with optional real Postgres/Redis.
- **Local development** — local hot reload and Docker Compose for Postgres, Redis, and observability services.
- **Agent rules** — AI coding guidance that encodes the template's layer boundaries and patterns.

## Architecture and Principles

These keep framework and infrastructure details at the edges so business logic stays independent and changes stay localized.

**Architecture & structure**

- [Clean Architecture](https://8thlight.com/insights/uncle-bob/2012/08/13/the-clean-architecture.html)
- [Dependency Injection](https://martinfowler.com/articles/injection.html) ([Composition root](https://blog.ploeh.dk/2011/07/28/CompositionRoot/))
- [SOLID principles](https://en.wikipedia.org/wiki/SOLID)
- [Consumer-owned interfaces](https://go.dev/doc/effective_go#interfaces_and_types)

**Structural patterns**

- [Adapter pattern](https://refactoring.guru/design-patterns/adapter)
- [Decorator pattern](https://refactoring.guru/design-patterns/decorator)
- [Facade pattern](https://refactoring.guru/design-patterns/facade)

**Behavioral & creational**

- [Builder pattern](https://refactoring.guru/design-patterns/builder)
- [Strategy pattern](https://refactoring.guru/design-patterns/strategy)
- [Null object pattern](https://en.wikipedia.org/wiki/Null_object_pattern)

**Data & transport**

- [Cache-aside pattern](https://learn.microsoft.com/en-us/azure/architecture/patterns/cache-aside)
- [Repository pattern](https://martinfowler.com/eaaCatalog/repository.html)
- [Middleware pattern](https://www.alexedwards.net/blog/making-and-using-middleware)

See package-level `README.md` files and [`AGENTS.md`](AGENTS.md) for implementation guidance and shared architecture rules for both engineers and AI agents.

## Third-Party Tools

**Transport**

- [`Echo v5`](https://github.com/labstack/echo) - HTTP server
- [`go-playground/validator/v10`](https://github.com/go-playground/validator) - request/DTO validation (struct tags)
- [`OpenAPI 3`](https://spec.openapis.org/oas/latest.html) - source of truth for HTTP contracts
- [`oapi-codegen`](https://github.com/oapi-codegen/oapi-codegen) - backend transport model generation from OpenAPI

**Database**

- [`pgx/v5`](https://github.com/jackc/pgx) - PostgreSQL driver and connection pool
- [`sqlc`](https://github.com/sqlc-dev/sqlc) - static SQL query generation
- [`Masterminds/squirrel`](https://github.com/Masterminds/squirrel) - dynamic SQL construction
- [`pressly/goose`](https://github.com/pressly/goose) - database migrations

Why SQL-first data access (no ORM)

- Raw SQL with [`sqlc`](https://sqlc.dev/) + [`squirrel`](https://github.com/Masterminds/squirrel) provides explicit query control, predictable performance tuning, and compile-time type safety.
- Common downsides are handled by:
  - `sqlc` generated typed mappings to reduce runtime schema/query mismatch risk.
  - `squirrel` composable dynamic SQL to avoid fragile string concatenation.
  - Clean Architecture + repository boundaries to isolate SQL in infra adapters and keep usecases storage-agnostic.

**Cache**

- [`go-redis/v9`](https://github.com/redis/go-redis) - Redis client integration

**Object storage**
- [`AWS SDK for Go v2`](https://aws.github.io/aws-sdk-go-v2/docs/) - S3-compatible Cloudflare R2 object-storage adapter

**Authentication**

- [`golang.org/x/oauth2`](https://pkg.go.dev/golang.org/x/oauth2) - OAuth 2.0 client support for Google login
- [`Resend`](https://resend.com/) - optional OTP email delivery

**Observability & logging**

- [`uber-go/zap`](https://github.com/uber-go/zap) - structured logging
- [`OpenTelemetry`](https://opentelemetry.io/) SDK + [`OTLP`](https://opentelemetry.io/docs/specs/otlp/) exporters - tracing, logs, and metrics
- [`HyperDX`](https://www.hyperdx.io/) + [`OpenTelemetry Collector`](https://opentelemetry.io/docs/collector/) - local observability integration

**Development & infra**

- [`air`](https://github.com/air-verse/air) - local hot reload
- [`Docker Compose`](https://docs.docker.com/compose/) - local infrastructure orchestration


## Requirements

- [Go 1.26+](https://go.dev/doc/install)
- [Docker](https://docs.docker.com/get-docker/) and [Docker Compose](https://docs.docker.com/compose/install/)
- [GNU Make](https://www.gnu.org/software/make/)

## Use as a Starter

1. Create a new repository from this template.
2. Bootstrap the project:

```bash
./scripts/bootstrap-template.sh --module github.com/your-org/your-backend
```

This updates module/import paths, service and stack naming, OpenAPI title, and README title.

3. If you cloned this repo directly, rename your project directory and set the Git remote to your new repository. The script does not change directory names or remotes.
4. Validate with `make openapi-generate && make test`.
5. Review `docker-compose.yml`, `.env.example`, and `docs/openapi.yaml` for project-specific values. The auth (pluggable OTP/OAuth + session), cached user store, and health modules are production-ready foundations—extend them and add your own migrations and features.
6. Review `AGENTS.md` and package-level `README.md` files before feature development.

## Setup and Run

1. `cp .env.example .env`
2. `make install`
3. `make dev-up && make migrate-up`
4. `make run`
5. Verify `GET /health?check=ready`

Common commands: `make dev-logs`, `make dev-down`, `make migrate-status`, `make openapi-generate`, `make run-stop`.

Default local ports: Postgres `5432`, Redis `6379`, OTel `4317/4318`, HyperDX `8081`.

## Verify

Run the unit and standard HTTP integration suites:

```bash
make test
make test-integration
```

With Postgres and Redis running, run the real-adapter integration suite. It loads a repository-root `.env` when present; explicitly exported environment variables take precedence.

```bash
make test-integration-real
```

To observe the bounded SSE demo while the API is running:

```bash
curl -N 'http://localhost:8080/health/stream?check=ready'
```
