---
description: SQL Rules
---

# SQL Rules

**sqlc** for static queries. **Squirrel** for dynamic queries. No string concatenation. All SQL lives in infra.

**No N+1 / no DB in loops:** Do not execute DB queries inside loops. Prefer window functions, joins, batch queries (`IN`/`ANY`), and bulk operations. If a write flow truly cannot be made set-based (e.g. IDs are generated and required for subsequent rows), use the smallest number of round-trips possible and document why it’s unavoidable.

**Type alignment across layers:** Keep repository/usecase primitive field types aligned with DB schema intent (e.g. `BIGINT` -> `int64`) to avoid repeated casts and silent narrowing. Do NOT expose vendor/driver-specific types (e.g. pgx/pgtype) outside infra; map them at the repository boundary.

