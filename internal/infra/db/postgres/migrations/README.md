# PostgreSQL migrations

This directory contains ordered SQL migrations executed by Goose to upgrade existing PostgreSQL databases.

## Responsibilities and boundaries

- Migrations create or transform schema, stored data, indexes, and required PostgreSQL features.
- A migration becomes immutable once another developer or persistent environment may have applied it.
- sqlc schema files describe generation input; migrations describe the upgrade path for running databases.

## Extending

- Add a new ordered migration for a released or shared change.
- Revise an existing migration only while the change is local and every affected database can be recreated.
- Mirror structural changes in the relevant `sqlc/schema` file and regenerate sqlc output.
