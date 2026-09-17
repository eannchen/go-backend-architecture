# Project

Go backend organized as a modular monolith with Clean Architecture. Keep business decisions independent of transport, storage, and framework details. Prefer explicit dependencies, small contracts, and code whose ownership is clear from its package.

The codebase may contain any combination of capabilities. Apply capability-specific rules only when the related source or contract is present, or when the requested change adds that capability. Do not restore a removed capability merely to follow an inapplicable rule.

# Architecture and dependencies

```text
delivery -> usecase
usecase -> repository contracts + domain
repository contracts -> domain
infra -> repository contracts + domain
delivery/usecase/infra -> shared technical contracts
app -> all layers (composition only)
```

- **domain** owns business entities, value objects, and reusable invariants. It imports neither usecase, repository, infra, nor delivery.
- **usecase** coordinates business workflows. It may depend on domain types, repository contracts, and shared technical contracts, but never on delivery or infra.
- **repository** owns outbound capability contracts required by usecases, including persistence, external providers, and replaceable infrastructure-backed operations. Contracts may use domain types but must not expose driver, framework, or vendor types.
- **shared technical contracts** live in purpose-specific packages such as `internal/logger`, `internal/observability`, and, when required, `internal/security`. They remain outside repository because they provide project-wide facilities rather than capabilities requested by a usecase.
- **infra** implements repository and shared technical contracts using databases, caches, external services, and third-party libraries. It maps infrastructure-specific types at the boundary.
- **delivery** owns inbound adapters. Current adapters validate HTTP requests and gRPC calls, invoke usecases, and map outcomes to transport responses. Delivery contains no persistence logic or business policy.
- **app** is the composition root. Only app chooses concrete implementations and wires dependencies across layers.

Do not bypass a layer to call another layer's implementation.

# Feature placement and naming

Create only the paths a feature requires: add domain for reusable business rules, repository and infra for outbound capabilities, and a delivery adapter only when the feature is exposed through it. Wire each present adapter in its matching composition files: HTTP in `internal/app/httpapi/httpapi_*_wiring.go` and gRPC in `internal/app/grpcapi/grpcapi_*_wiring.go`.

```text
internal/domain/<concept>/                         # only when shared business meaning exists
internal/usecase/<feature>/
internal/repository/<area>/<feature>_repository.go
internal/infra/<area>/<backend>/store/<feature>_store.go
internal/delivery/http/handler/<feature>/             # when HTTP is present
internal/delivery/grpc/service/<feature>/             # when gRPC is present
```

- Store implementations live under backend-specific paths such as `db/postgres/store`, `cache/redis/store`, and `kvstore/redis/store`.
- Cross-backend decorators such as cache-aside stores live under `internal/infra/composed/<feature>/`.
- Name files so their purpose is visible in an editor tab: `<feature>_handler.go`, `<feature>_usecase.go`, and `<feature>_repository.go`.
- Middleware and interceptors use `<feature>_middleware.go` or `<feature>_interceptor.go`; supporting files name the concern they implement.
- Keep packages cohesive. Do not create generic `common`, `helpers`, or `utils` packages for code without a clear owner.

# Construction, contracts, and types

- Use constructor injection. Do not use service locators, mutable global dependencies, or package initialization for runtime wiring.
- Use the existing contract packages for architectural boundaries. Do not redefine repository, usecase, logger, observability, or security interfaces elsewhere.
- Use a consumer-owned interface only for a package-specific dependency not represented by an existing architectural contract. Define it beside the code that uses the dependency and include only the methods that code needs.
- Keep interfaces small and behavior-focused. Add methods for usecase needs, not CRUD completeness.
- Introduce a repository contract when the application needs replaceable behavior or isolation from infrastructure details; do not wrap a third-party package solely because it is external code.
- For HTTP, transport DTOs and their `json`, `query`, `form`, normalization, and validation tags remain in delivery. Map them to usecase or domain types before crossing the boundary.
- Generated OpenAPI or Protobuf types remain in their respective delivery adapters and must not appear in usecase, domain, or repository APIs.
- Keep primitive types aligned with schema intent, such as PostgreSQL `BIGINT` to Go `int64`. Map driver-specific types inside infra.
- Configuration is typed, validated during startup, and injected. Do not read environment variables throughout business or delivery code.

# Input and business validation

- Delivery validates transport shape, encoding, required fields, and portable request constraints.
- Domain or usecase code validates business invariants and authorization decisions.
- Use database constraints or atomic database operations for rules that must remain correct under concurrency; do not rely on check-then-write logic.
- For HTTP, use the injected Echo binder. Do not manually repeat trimming or case normalization already expressed by binding tags.

# Context, concurrency, and lifecycle

- Accept `context.Context` as the first parameter of operations that perform I/O or may block. Propagate it to downstream calls and do not store it in structs.
- Do not replace an incoming context with `context.Background()`. Use a new root context only for independently owned application lifecycle work, and give cleanup work an explicit timeout.
- Preserve cancellation and deadline errors through wrapping so the transport responder can map them correctly.
- Bound outbound network calls with deadlines at the client or application boundary.
- Retry only documented transient failures. Use bounded attempts and backoff, stop when the context ends, and do not retry non-idempotent operations without an idempotency design.
- Do not start a goroutine without defined ownership, cancellation, panic handling where needed, and a way to wait for completion.
- Bound parallel work with a worker limit or semaphore; never create goroutines directly from unbounded input.
- Shutdown owners in reverse dependency order. Handle every cleanup error and combine independent shutdown failures with `errors.Join`.

# Data access, consistency, and performance

- For SQL adapters, use **sqlc** for static SQL and **Squirrel** for dynamic SQL. All SQL stays in infra; never build SQL by concatenating values.
- Avoid N+1 access. Use joins, batch operations, `IN`/`ANY`, window functions, or a repository method shaped around the usecase.
- Prefer one storage round trip for related reads. Split a read only when data is optional, the complexity reduction is material, or stable reusable data benefits from a different cache and freshness policy than volatile data; document why the extra round trips are worthwhile.
- Every list and batch operation must be bounded. Use keyset/cursor pagination for large or frequently changing datasets; justify offset pagination where its cost can grow.
- Select only data required by the repository result. Do not load complete rows or relations for convenience on hot paths.
- When several writes must be atomic, expose one repository capability describing the operation. The infra implementation owns the transaction; transaction handles never cross into usecase code.
- Map vendor errors to repository sentinels with `errors.Join`, allowing usecases to use `errors.Is` without losing the original cause.
- Add indexes for actual query access patterns. For material query changes, inspect the query plan rather than assuming an index or rewrite is faster.
- Caches are optional performance layers. For each cache, define its pattern, such as cache-aside or write-through, its acceptable staleness, and its behavior when reads, writes, or invalidation fail. Choose fallback, retry, bypass, or failure according to the feature's consistency and availability requirements.
- Prefer clear code over speculative micro-optimization. Benchmark or profile before adding complexity, but preallocate collections when the final size is already known and no extra pass is required.

# Errors, security, and observability

- Do not discard errors silently. Return them with useful context, convert them to the current layer's error type, or log and continue when the failure is non-fatal. If an error is intentionally ignored, comment why that is safe.
- Infra returns failures with details about the failed operation and preserves the cause with `%w`. Usecases convert dependency failures to `apperr`; each delivery adapter performs its final error mapping.
- Define stable sentinel errors in the appropriate repository area. Inspect errors with `errors.Is` or `errors.As`, never string matching.
- Do not panic while handling an HTTP request, gRPC call, broker message, or background job. Reserve panics for unrecoverable programmer errors during startup or invariant violations that cannot be returned.
- Log an unexpected failure once at the layer that owns the handling decision. Do not log it again in delivery after a responder has handled it.
- When an optional dependency fails and processing continues without it or through another dependency, log a warning naming the failed dependency, affected operation, and how processing continued.
- Never log credentials, session tokens, OTPs, authorization metadata, cookies, private keys, or unredacted sensitive payloads.
- Authentication establishes caller identity at the delivery boundary; usecases enforce business authorization. Missing or ambiguous identity must fail closed where identity is required.
- Add structured log fields with `logger.Fields` instead of embedding values in message strings.
- Only packages under `internal/infra/observability` may import OpenTelemetry; other packages use the contracts in `internal/observability`.
- Metric attributes must be bounded. Put concrete paths, raw errors, IDs, and detailed diagnostics in traces or logs rather than metric dimensions.
- Add spans around meaningful I/O or expensive work, not every small function. Preserve trace context across supported internal calls and messages.

# Testing

- Test behavior at the layer that owns it. Keep the subject real and replace only dependencies outside that test's scope.
- Usecase tests replace repositories and assert business rules and error mapping. Delivery tests replace usecases and assert adapter-specific input handling, state changes, and outputs.
- Add regression coverage in the package where a defect originated: persistence behavior in the adapter test, response behavior in delivery, and business behavior in the usecase or domain test.
- Keep unit tests beside their source. Name integration files `<subject>_integration_test.go` and guard them with the `integration` build tag.
- Use table-driven tests when cases share one arrange/act/assert flow. Keep stateful and multi-step workflows explicit.
- Add one canonical configurable double under the contract owner's `xxxtest` package when first needed. Use `<Method>Func` for behavior and `<Method>Calls` for observations; unconfigured calls panic.
- Use a real disposable backend for infrastructure integration tests. Prefer Testcontainers when the dependency has a suitable container image. Start one instance of each required backend per package, terminate it explicitly, and clean each test's data. Fail rather than skip when the backend is unavailable.
- Do not use a fixed sleep to assume concurrent work has finished. Wait for a channel signal, use a controllable clock, or wait for a real state change with a timeout.
- Do not chase coverage on generated code or trivial pass-throughs.
- Run focused tests first. Before handoff, run checks proportional to the change: `make test`, `make test-integration`, `make test-all`, and `-race` for concurrency-sensitive code.

# Transport contracts and generated code

## HTTP and OpenAPI

- When HTTP is present, `contracts/http/openapi.yaml` is the source of truth for endpoint purpose and field meaning. Every endpoint has `summary` and `description`; every request and response field has `description`.
- Express portable constraints in OpenAPI and Go-specific binding, normalization, or validator tags with `x-oapi-codegen-extra-tags`.
- After changing the contract, run `make openapi-generate`, then adapt delivery mappings and tests.

## gRPC and Protobuf

- When gRPC is present, versioned files under `contracts/grpc/` are the source of truth for its contracts. Preserve existing field numbers; reserve removed field numbers and names.
- After changing a contract, run `make proto-lint proto-generate`, then adapt services and tests.

For either transport, never edit generated files directly. Review generated diffs and ensure regeneration is clean and deterministic.

# Database migrations

- Use migrations for schema, stored-data, and index changes that must be applied to existing databases.
- Revise a migration only while its change is local and every affected database can be safely recreated; otherwise add a new migration.
- When a database change affects schema definitions, indexes, queries, generated data-access code, or adapter mappings, update all affected files in the same change.

# HTTP JSON response semantics

These rules apply only to HTTP response DTOs serialized as JSON, including generated OpenAPI response models. They do not define Protobuf or event-message presence semantics. Internal domain, usecase, and repository types may use idiomatic Go representations until delivery maps them.

- Every schema-defined response field is present unless the contract explicitly makes it inapplicable.
- `null` means the value applies but is unknown or unavailable. It never means "not loaded yet."
- Unknown strings and numbers use `null`, not empty strings or zero placeholders. Booleans always resolve to `true` or `false`.
- Empty arrays serialize as `[]`, never `null`.
- An absent object is `null`; `{}` means the object exists but has no properties.
- Do not use omission and `null` interchangeably for the same field.

# Documentation and change discipline

- Read README files that belong to the affected subsystem; the root README is user-facing and is not a development guide. Search for existing patterns before introducing a new abstraction.
- Add a README only for a meaningful architectural boundary, extension point, lifecycle, generated-code workflow, or non-obvious subsystem.
- A README starts with the component's purpose, then documents responsibilities, boundaries, and extension guidance. Add security, lifecycle, generation, or verification details only when relevant.
- Keep documentation concise, concrete, and unambiguous; omit unnecessary detail. Keep rules that apply across multiple subsystems, such as dependency direction, testing, error handling, and naming, in `AGENTS.md` instead of repeating them in subsystem READMEs. Update documentation when behavior, ownership, or extension steps change.
- Match scope to intent. For normal feature work, refactor touched code when its current design obstructs a clean implementation. For an explicitly requested hotfix, prefer the smallest correct change.
- Extend existing patterns when they fit; introduce a new pattern only when the repository has no suitable seam.
- Comment non-obvious reasons, business rules, concurrency, tradeoffs, and complex or low-level mechanisms. Describing what the code does is appropriate when the implementation is difficult to follow; do not paraphrase straightforward code.
- Prefer idiomatic Go and straightforward control flow. Avoid redundant passes over data unless the separation materially improves clarity.
