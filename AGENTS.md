# AGENTS.md

Single source of truth for AI rules. Derived files generated for `.cursor/rules/` and `.claude/rules/`. After changes, run `./scripts/sync-ai-rules.sh`.

---

# Project Overview

Go modular-monolith backend template with Clean Architecture. SOLID principles enforced through layer boundaries, consumer-owned interfaces, constructor injection, and repository contracts.

---

# Architecture Layers

```
delivery -> usecase -> repository contracts
infra -> repository contracts
app -> wires everything together
```

- **delivery** — Transport only: handlers, validation, response mapping.
- **usecase** — Business logic, independent of frameworks.
- **repository** — Contracts (interfaces) for usecases. Subdirs mirror infra: `db/`, `cache/`, `kvstore/`, `external/`.
- **infra** — Implements contracts: postgres, redis, external services, logger, observability. `composed/` holds decorator stores that combine multiple implementations (e.g. cache-aside).
- **app** — Composition root: wiring, adapters, server startup.

---

# Dependency Rules

**Allowed:** `delivery -> usecase`, `usecase -> repository`, `infra -> repository`, `app -> all`.

**Forbidden:** usecase must NOT import infra or delivery; repository must NOT import infra. Only `internal/app` may import across layers.

---

# Feature Structure

Create in order, then wire in `internal/app/wiring.go`:

```
internal/usecase/<feature>/
internal/repository/<area>/<feature>_repository.go
internal/infra/<area>/<backend>/store/<feature>_store.go
internal/delivery/http/handler/<feature>/
```

Store implementations live under a backend-specific path (e.g. `db/postgres/store`, `cache/redis/store`, `kvstore/redis/store`). When a feature needs a composed store (e.g. cache-aside), add it under `internal/infra/composed/<feature>/`.

---

# File and directory naming

Names should make **purpose visible from the editor tab**.

- **Handlers:** `handler/<feature>/` with `<feature>_<role>.go` (e.g. `auth_handler.go`, `auth_dto.go`, `health_handler.go`).
- **Middleware:** `<feature>_middleware.go`, `<feature>_<specific>_middleware.go`; support files without `_middleware` (e.g. `observability_keys.go`); tests `<feature>_middleware_test.go`.
- **Usecase:** `<feature>_usecase.go` with interface + impl in one file. Multi-capability features use subdirs (e.g. `auth/otp/otp_usecase.go`); shared types in the parent (`auth_types.go`).
- **Repository:** `xxxx_repository.go` in the matching subdir (`db/`, `cache/`, `kvstore/`, `external/`).

---

# Constructor Injection

Constructor injection only. No service locator or global containers. Dependencies must be explicit.

---

# Interfaces

Consumer-owned: define where used. Keep small. Add only for multiple implementations, test seams, or cross-layer contracts.

---

# DTO Rules

Transport DTOs (with `json`, `query`, `form`, `validate` tags) belong in delivery only. Usecase models must NOT contain transport tags. Map between DTOs and usecase models in delivery.

---

# Request binding and normalization

Pluggable `echo.Binder` injected into the server. Default: `binding.NewNormalizeBinder(nil)` — trims whitespace on bind; optional `case:"lower"` / `case:"upper"` / `trim:"false"` struct tags on DTOs. No manual trim/case in handlers. OpenAPI-generated models in `openapi/gen` are for response mapping; keep request DTOs in delivery for binding tags.

---

# SQL Rules

**sqlc** for static queries. **Squirrel** for dynamic queries. No string concatenation. All SQL lives in infra.

**No N+1 / no DB in loops:** Do not execute DB queries inside loops. Prefer window functions, joins, batch queries (`IN`/`ANY`), and bulk operations. If a write flow truly cannot be made set-based (e.g. IDs are generated and required for subsequent rows), use the smallest number of round-trips possible and document why it’s unavoidable.

**Type alignment across layers:** Keep repository/usecase primitive field types aligned with DB schema intent (e.g. `BIGINT` -> `int64`) to avoid repeated casts and silent narrowing. Do NOT expose vendor/driver-specific types (e.g. pgx/pgtype) outside infra; map them at the repository boundary.

---

# Error Handling

Usecases return `apperr.New`/`apperr.Wrap`; handlers convert to transport responses. Infra returns `fmt.Errorf` with `%w`; usecases wrap at the boundary. All errors must be handled — log non-fatal ones at warn level minimum.

**Sentinel errors:** Define in repository per area (e.g. `repository/db/errors.go`: `ErrDuplicateKey`). Infra maps vendor errors with `errors.Join(sentinel, err)`; usecase uses `errors.Is(err, repo.ErrX)` and returns the right `apperr` code.

---

# HTTP Observability

Routes register via `RouteRegistrar`. Middleware provides tracing. Add spans where important. Do not import OpenTelemetry outside observability packages.

---

# Logging

Structured logging (`logger.Fields(...)`). Log meaningful events only. Handlers should not log errors already handled by response helpers.

---

# Coding Style

Idiomatic Go. Exported `PascalCase`, unexported `camelCase`, constructors `NewX(...)`. Interfaces describe behavior; avoid `I*` prefixes. Small functions; named returns only when they improve clarity.

---

# Commenting Rules

Explain **why**, not **what**. Add comments for business rules, non-obvious decisions, concurrency/caching. Skip obvious code. Exported functions: one short line.

---

# Documentation Standards

Each package README has **Pattern used** and **How to extend** only. Short, architecture-focused. No duplication across docs. Update when outdated.

---

# OpenAPI Rules

`docs/openapi.yaml` is the single source for API purpose and field meaning. Every endpoint needs `summary` + `description`; every input/response field needs `description`. After changes: `make openapi-generate`, then regenerate `docs/insomnia.json`.

---

# AI Agent Guidelines

1. Search repo for existing patterns first.
2. Follow architecture and dependency boundaries.
3. Prefer modifying existing structures over new abstractions.
4. Use same constructor and wiring patterns.
5. Match commenting style; skip unnecessary comments.
6. For HTTP changes: update `docs/openapi.yaml` first, run `make openapi-generate`, then adapt handlers.
7. Use binding tags on DTOs (`trim:"false"`, `case:"lower"`, `case:"upper"`); no manual trim/case.
8. Follow **File and directory naming** conventions above.
