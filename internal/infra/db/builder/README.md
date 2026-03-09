# internal/infra/db/builder

Dynamic SQL builder used by repository implementations when query shape depends on runtime conditions.

## Pattern used

- Shared builder configuration and placeholder format; used only from infra store packages.
- No static or generated SQL in this package; no business logic—only SQL-shape utilities (filters, sort, paging).
- Aligns with project SQL rules: static queries elsewhere, dynamic shape here.

## How to extend

- Add reusable builder helpers (e.g. common filter or paging fragments) here.
- Do not add business rules or static SQL; keep generated/static SQL in the appropriate store and SQL layer.
