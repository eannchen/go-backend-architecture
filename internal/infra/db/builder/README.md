# internal/infra/db/builder

Dynamic SQL builder setup.

## Pattern used

- Builder pattern for dynamic SQL construction.
- Centralizes shared `Squirrel` configuration (placeholder format, defaults).
- Used by repositories when query shape is conditional at runtime.

## How to extend

- Add reusable builder helpers here (sorting/filter/paging snippets).
- Keep generated/static SQL (`sqlc`) out of this package.
- Do not add business logic here; this package is SQL-shape utilities only.
