# internal/infra/db/builder

## Pattern used

- Shared builder config and placeholder format for dynamic SQL; used only from infra store packages.
- No static/generated SQL or business logic — only SQL-shape utilities (filters, sort, paging).

## How to extend

- Add reusable builder helpers (filter fragments, paging) here.
- Keep generated/static SQL in the appropriate store and SQL layer.
