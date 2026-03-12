# AGENTS.md

This file defines **architecture rules, coding conventions, and AI-agent guidance** for this repository. It is written to be readable by humans, enforceable by AI agents, and minimal but precise.

**Single source of truth** for AI rules. Derived files are generated for `.cursor/rules/` and `.claude/rules/`. After changes, run:

```
./scripts/sync-ai-rules.sh
```

---

# Project Overview

**Go backend architecture template** implementing a **modular monolith with Clean Architecture**. Goals: strong dependency boundaries, clear separation of business logic and infrastructure, testable usecases, predictable structure, and AI agents generating correct code consistently.

Design follows **SOLID principles**: layer boundaries and small functions (SRP); extend via new code and consumer-owned interfaces (OCP); interfaces imply substitutability (LSP); small, consumer-owned interfaces (ISP); depend on repository contracts and constructor-injected abstractions (DIP). See Architecture Layers, Dependency Rules, Constructor Injection, and Interfaces below.

---

# Architecture Layers

Dependency direction:

```
delivery -> usecase -> repository contracts
infra -> repository contracts
app -> wires everything together
```

### delivery

Transport only: HTTP handlers, request validation, response mapping. No business logic.

### usecase

Business logic: orchestration, domain rules, calling repositories. Independent of frameworks and infrastructure.

### repository (contracts)

Interfaces used by usecases (e.g. `AccountRepository`, `TxManager`, `CacheStore`). No database code here.

### infra

Implements repository contracts: postgres, redis, logger, observability, external services. Depends on contracts only.

### app

Composition root: dependency wiring, adapters between packages, starting the server. Cross-package wiring happens here only.

---

# Dependency Rules

**Allowed:** `delivery -> usecase`, `usecase -> repository`, `infra -> repository`, `app -> all layers`.

**Forbidden:** usecase must NOT import `internal/infra` or delivery; repository must NOT import infra; domain must NOT import outer layers. Only `internal/app` may import across layers.

---

# Feature Structure

When adding a feature, create in order:

```
internal/usecase/<feature>/usecase.go
internal/repository/<feature>.go
internal/infra/db/postgres/store/<feature>.go
internal/delivery/http/<feature>/  (handler.go, dto.go)
```

Then wire in `internal/app/wiring.go`. Do not skip layers.

---

# Constructor Injection

Use constructor injection only. No service locator or global containers. Dependencies must be explicit.

Examples:

- Handler: `NewHandler(log, tracer, usecase)`
- Usecase: `New(log, tracer, repo)`

---

# Interfaces

Prefer **consumer-owned interfaces**: define the interface where it is used. Example: server depends on `RouteRegistrar`, not concrete handlers. Keep interfaces small. Add interfaces only for multiple implementations, test seams, or cross-layer contracts. Avoid unnecessary abstraction.

---

# DTO Rules

Transport DTOs (with `json`, `query`, `form`, `validate`) belong in **delivery only**. Usecase models must NOT contain transport tags. In delivery, map between DTOs and usecase models.

When OpenAPI is used, treat `docs/openapi.yaml` as the external contract for frontend repos and AI agents. Generated models in `internal/delivery/http/openapi/gen` are mainly for shared contract types and response mapping. Keep request DTOs in delivery when Echo binding or `validator/v10` depends on delivery or usecase parsing rules.

---

# SQL Rules

Use **sqlc** for static queries (SELECT, INSERT, UPDATE). Use **Squirrel** for dynamic queries (optional filters, complex conditions). Do not build SQL via string concatenation. All SQL lives in infra.

---

# Error Handling

Usecases return application errors from `internal/apperr` (`apperr.New`, `apperr.Wrap`); handlers convert them to transport responses. Infra returns standard Go errors (`fmt.Errorf` with `%w`); usecases wrap infra errors into app errors at the boundary. Handlers must NOT return raw errors.

---

# HTTP Observability

All routes register via `RouteRegistrar`. Middleware provides request-level tracing. Add spans in handlers, usecases, and repositories where important. Do not import OpenTelemetry outside observability packages; use observability interfaces.

---

# Logging

Use structured logging (`logger.Fields(...)`). Log meaningful events only. Handlers should not log business errors already handled by response helpers.

---

# Coding Style

Idiomatic Go. Naming: exported `PascalCase`, unexported `camelCase`, constructors `NewX(...)`. Interfaces describe behavior (`Usecase`, `RouteRegistrar`, `TxManager`); avoid `I*` prefixes. Keep functions small; use named returns only when they improve clarity.

---

# Commenting Rules

Explain **why**, not **what**. Avoid repeating what the code already states.

- **Add comments when:** business rules, non-obvious design decisions, concurrency/caching/retry, assumed external behavior.
- **Avoid comments for:** obvious assignments, simple control flow, basic constructs.
- **Exported functions:** one short line describing intent. Keep to 1–2 lines.

---

# Documentation Standards

Short, accurate, architecture-focused. Each package `README.md` includes **Pattern used** and **How to extend**. Prefer concept over file-by-file narration. Update docs as soon as they become outdated.

- **Tone:** Technical and structure-oriented. Explain patterns, boundaries, and where types live rather than step-by-step or concrete usage examples.
- **Concrete examples:** Omit file names, vendor names, and example code unless they are necessary to explain the structure or avoid ambiguity.
- **Avoid duplication:** Do not repeat the same explanation across documents, including this file, unless the repetition adds needed context at that location.

---

# OpenAPI Explanation Rules

Treat `docs/openapi.yaml` as the single place that describes the **purpose of each API** and the **meaning of their fields** for frontends and API consumers.

When updating `docs/openapi.yaml`, keep explanations short and consistent:

1. Add `summary` and `description` for every endpoint; the description must state the **purpose** of the API (why it exists and how it is used).
2. Add `description` for every user input (header, path, query, and request body fields) and for every response schema and important field.
3. Describe domain meaning and behavior, not implementation details.
4. Keep each description to 1–2 lines; avoid repeating obvious type information.
5. After OpenAPI changes, run `make openapi-generate`.

---

# AI Agent Guidelines

When generating code:

1. Search the repo for existing patterns before creating new ones.
2. Follow the existing architecture and dependency boundaries.
3. Prefer modifying existing structures over new abstractions.
4. Use the same constructor and wiring patterns as the codebase.
5. Match commenting style of nearby code; add comments for complex logic, skip unnecessary ones.
6. For HTTP contract changes, update `docs/openapi.yaml` first, run `make openapi-generate`, and then adapt delivery handlers.

The architecture should stay predictable for humans, AI agents, and future maintainers.
