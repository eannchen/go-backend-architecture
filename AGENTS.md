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

Create in order, then wire in the matching process composition file (for the HTTP API, `internal/app/api/api_*_wiring.go`):

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

# Change Scope

Keep the project's template structure — layers, feature layout, wiring. Extend existing patterns when they fit; add a new one only when the template has no seam for it.

Match implementation scope to intent:

- **Default:** idiomatic Go within that structure. Refactor what you touch when implementation fights the change — extract, rename, simplify; don't work around awkward code.
- **Hotfix** (user asks for minimal/urgent fix): smallest correct diff; defer cleanup unless it blocks the fix.

---

# Testing

Test behavior at the layer that owns it. Keep the subject real; replace only dependencies outside the test scope.

## Boundaries

| Scope | Keep real | Replace | Assert |
| --- | --- | --- | --- |
| Usecase unit | Usecase | Repositories | Business rules and error mapping |
| Delivery unit | Handler/middleware, binder, validator, responder | Usecases | Normalization, validation, cookies/headers, status, response mapping |
| Utility unit | Utility | Only dependencies it crosses | Public behavior and edge cases |
| Infra adapter integration | Adapter and Testcontainers backend | Unrelated external boundaries | SQL/Redis semantics, persistence, serialization, TTL, atomicity |
| HTTP feature integration | Server through delivery, usecase, repository, and infra | External providers outside the feature | Client-visible workflows and responses |
| Composition/lifecycle | Wiring and lifecycle orchestration | Expensive infrastructure outside the decision | Configuration, ordering, cleanup, error aggregation |

Test technical details in the layer that implements them. Feature tests check HTTP workflows and responses; PostgreSQL adapter tests check SQL behavior; Redis adapter tests check keys, serialization, TTL, and atomicity.

## Structure and Style

- Keep unit tests beside their source. Prefer one matching `<source>_test.go` file per source file.
- Name integration files `<subject>_integration_test.go` and use the `integration` build tag.
- Under `internal/delivery/http/integration`, put shared HTTP mechanics in `server_fixture_integration_test.go`, feature wiring/cleanup in `<feature>_fixture_integration_test.go`, and client workflows in `<feature>_flow_integration_test.go`.
- Name tests with the subject and expected result, such as `TestGetHealthRejectsInvalidQuery`. The name should explain the behavior without requiring the reader to inspect the test body.
- Use table-driven tests when cases share one arrange/act/assert flow; keep stateful or multi-step workflows explicit.
- For every bug fix, add a test in the package where the incorrect behavior originated: SQL bug -> PostgreSQL adapter test; wrong HTTP response -> handler test.
- Do not chase coverage on generated code or trivial pass-through code.

## Test Doubles

- When first needed, add one canonical configurable double for each cross-package interface under its owner's `xxxtest` child package. Do not duplicate or create doubles speculatively.
- Name it after the interface (`UserRepository`, not `MockUserRepository`). For a method such as `GetByID`, use `GetByIDFunc` to configure behavior and `GetByIDCalls` to record calls. Store arguments only when a test needs them; use typed call records when every call matters.
- Unconfigured method calls must panic so missing test setup fails immediately.
- Configure different outcomes on the canonical double. Add another type only for a distinct reusable model such as `InMemoryUserRepository`. Define a double inside one test file only for an unexported interface in that package or a special edge case that will not be reused, such as a response writer that fails on `Write`.

## Integration Lifecycle

- Use Testcontainers, not database or Redis endpoints from the environment.
- Start one container of each required type per package, normally in `TestMain`, and terminate it explicitly after the suite.
- Each test must remove its own rows and keys. Container termination cleans package resources; per-test cleanup prevents shared-container conflicts.
- Fail, never silently skip, when Docker or a required container is unavailable.

## Commands

- Focused: `go test ./path/to/package`; add `-race` for concurrency-sensitive changes.
- Default: `make test`; integration: `make test-integration`; both: `make test-all`.

---

# Commenting Rules

Explain **why**, not **what**. Plain language.

Comment business rules, non-obvious decisions, concurrency/caching. Skip restating the code. Keep comments short and scannable; a brief paragraph or structured list is fine when it aids clarity. Exported functions: one-line doc comment max. No commented-out code.

---

# Documentation Standards

Each package README has **Pattern used** and **How to extend** only. Short, architecture-focused. No duplication across docs. Update when outdated.

---

# OpenAPI Rules

`docs/openapi.yaml` is the single source for API purpose and field meaning. Every endpoint needs `summary` + `description`; every input/response field needs `description`. After changes: `make openapi-generate`, then adapt handlers.

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
4. Keep the template structure (layers, layout, wiring). Extend existing patterns; add new ones only when the template has no seam. Within it, write idiomatic Go — refactor awkward implementation; don't work around it. Hotfix (user asks minimal/urgent): smallest correct diff only.
5. Use same constructor and wiring patterns.
6. Comment why, not what. Keep comments short and scannable; skip restating code.
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

