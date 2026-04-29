<!--
  Source of truth for AI rules (edit above the generated Skills block).
  Skills source of truth: .agents/skills/<name>/SKILL.md
  After any edit, run ./scripts/sync-agents.sh to propagate changes to all tools.
-->

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

Use the project's contract packages for cross-layer boundaries; do not redefine them in the consumer.

- `internal/repository/db`, `cache`, `kvstore`, and `external` own dependency contracts used by usecases.
- `internal/usecase/...` owns business contracts used by delivery and app.
- `internal/logger` and `internal/observability` own their shared contracts.
- Consumer-owned interfaces are for local composition seams inside a layer (for example `RouteRegistrar` in delivery), not for rewriting repository or usecase contracts.

Keep interfaces small and behavior-focused. If a new boundary is needed, add it to the appropriate contract package instead of creating an ad hoc duplicate in the consumer.

---

# DTO Rules

Transport DTOs (with `json`, `query`, `form`, `validate` tags) belong in delivery only. Usecase models must NOT contain transport tags. Map between DTOs and usecase models in delivery.

---

# Request binding and normalization

Pluggable `echo.Binder` injected into the server. Default: `binding.NewNormalizeBinder(nil)` — trims whitespace on bind; optional `case:"lower"` / `case:"upper"` / `trim:"false"` struct tags on DTOs. No manual trim/case in handlers. OpenAPI-generated models in `openapi/gen` are for response mapping; keep request DTOs in delivery for binding tags.

---

# SQL Rules

**sqlc** for static queries. **Squirrel** for dynamic queries. No string concatenation. All SQL lives in infra.

**No N+1:** never run DB queries in loops. Prefer JOINs, window functions, and batch ops (`IN`/`ANY`). If multiple writes are unavoidable, minimize round-trips and document why.

**Usecase-oriented queries:** don’t chain repo calls for related data (Get A → then Get B). Instead, prefer a single query (JOIN/batch) via a dedicated repo method.

**Single round-trip (reads):** prefer one DB call. Multiple calls only if data is optional/rare or complexity reduction is significant; add a comment to justify.

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

**Iteration:** Prefer **one pass** over the same collection when it stays clear (merge derivations, batch SQL, pre-size from known `len`).

---

# Testing Rules

Test behavior at the layer that owns it.

- Business logic and utilities default to table-driven tests. Use explicit tests for infra adapters, HTTP transport/routing, and orchestration-heavy flows where setup differs materially per case.
- New business logic in `usecase` should ship with tests. Handler tests stay focused on bind/normalize/validate, cookie/header behavior, and response mapping.
- Prefer real integration tests for SQL, Redis, persistence, serialization, and external API adapters. Manual verification is fine during development, but important behavior should end up covered by automated integration tests.
- Every bug fix adds a regression test at the layer where the bug lived.
- Reusable test doubles live near the contract in test-only helper packages such as `dbtest`, `cachetest`, `kvstoretest`, `oauthtest`, `otptest`, `loggertest`, and `observabilitytest`. Keep one-off doubles local to the test file.
- Do not duplicate the same stub/fake across test files, and do not chase coverage on generated code or trivial pass-through code.

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

# JSON Field Semantics

These rules apply only to HTTP **response** DTOs (types serialized to JSON for clients, including OpenAPI-generated response models), not to repository, usecase, or other internal structs, which may use idiomatic Go (e.g. nil slices) until mapped at the delivery boundary.

All fields defined in the schema must always be present in the response. Never omit a field silently.

**Null** means the value is genuinely unknown or unavailable server-side. Use it sparingly and document which fields can be null.

**Type-specific defaults:**
- `string` → `null` if unknown; never use `""` unless it's a meaningful empty string
- `number` → `null` if unknown; never use `0` as a placeholder
- `boolean` → never `null`; always resolve to `true` or `false`
- `array` → `[]` if empty; never `null`
- `object` → `null` if the whole sub-resource is absent; `{}` only if the object exists but has no properties

**Never use null to mean "not loaded yet"** — that is client state, not API state.

**Omitted key vs. null value** are not interchangeable. A missing key means "this field doesn't apply to this response shape." A `null` value means "this field applies, but has no value." Pick one per field and stay consistent.

---

# AI Agent Guidelines

1. Read the `README.md` in any package directory before modifying it — it contains the pattern used and how to extend.
2. Search repo for existing patterns first.
3. Follow architecture and dependency boundaries.
4. Prefer modifying existing structures over new abstractions.
5. Use same constructor and wiring patterns.
6. Match commenting style; skip unnecessary comments.
7. For HTTP changes: update `docs/openapi.yaml` first, run `make openapi-generate`, then adapt handlers.
8. Use binding tags on DTOs (`trim:"false"`, `case:"lower"`, `case:"upper"`); no manual trim/case.
9. Follow **File and directory naming** conventions above.
10. Avoid redundant passes over the same data unless clarity or separation is worth it.

<!-- SKILLS: generated by sync-agents.sh — do not edit below this line -->

# Skills

Available runbooks in `.agents/skills/`. Read the steps below before performing each task.

## sync-agents


### Sync AI agent configuration

After changing `AGENTS.md` or any `.agents/skills/*/SKILL.md`, run the sync script to propagate changes to all tools.

#### Steps

1. Run from the repository root:
   ```bash
   ./scripts/sync-agents.sh
   ```
2. Confirm the script printed "Done. Claude Code, Cursor, and Codex are in sync."

No other steps. The script regenerates `.cursor/rules/`, `.claude/rules/`, and the `# Skills` block in `AGENTS.md`.

## sync-openapi-insomnia


### Sync OpenAPI and Insomnia

After changing `docs/openapi.yaml`, regenerate API artifacts so `docs/insomnia.json` stays aligned with the OpenAPI source of truth.

#### Steps

1. Run from the repository root:
   ```bash
   make openapi-generate
   ```
2. Regenerate `docs/insomnia.json` from the updated OpenAPI spec using the project's Insomnia export flow.
3. Verify both files reflect the intended update:
   - `docs/openapi.yaml`
   - `docs/insomnia.json`

Do not manually edit `docs/insomnia.json`; always regenerate it from the OpenAPI spec.

