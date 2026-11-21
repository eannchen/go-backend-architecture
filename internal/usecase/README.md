# Usecases

This directory contains application workflows exposed to delivery adapters.

## Responsibilities and boundaries

- A usecase coordinates domain rules and its outbound dependencies without importing delivery or infrastructure.
- A small workflow uses `internal/usecase/<feature>`; related workflows may use subpackages and share types in their parent package.
- Usecases return application errors that delivery can map without exposing infrastructure details.

## Extending

- Return a concrete workflow implementation from its constructor. Define outbound interfaces and their types in `<feature>_dependencies.go`; delivery defines the workflow interface it consumes.
- Keep shared business invariants in domain types and transport validation in delivery.
- Add dependency operations shaped around the workflow. Closely related workflows may share a contract in their parent package.

Usecase tests construct the real workflow and replace its outbound dependencies.
Use external test packages by default. Reusable doubles live under the interface
owner’s `<owner>test` package; callers in other test packages may reuse them.
