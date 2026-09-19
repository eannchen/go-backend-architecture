# PostgreSQL sqlc sources

This directory contains the SQL schema and queries used to generate the PostgreSQL data-access layer.

## Responsibilities and boundaries

- Schema and query files are editable generation inputs; `gen` contains generated Go code.
- Profile-specific sqlc configuration selects the schema and query files present in that project shape.
- Generated database types remain inside infrastructure adapters.

## Extending

- Add or change schema and query files, then run `make sqlc-generate`.
- Review generated changes and update store mappings in the same change.
- Keep runtime-conditional query construction in the SQL builder rather than concatenating SQL here.
