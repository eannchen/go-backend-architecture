# PostgreSQL migrations

## Pattern used

- Goose migrations apply only database features required by the selected template profile.
- Optional PostgreSQL extensions belong in feature-owned migrations when an application adopts them.

## How to extend

- Add a new ordered migration; do not edit a migration already applied outside local development.
- Keep structural changes aligned with the matching file under `sqlc/schema/`.
