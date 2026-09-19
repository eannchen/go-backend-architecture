# Usecases

This directory contains application workflows exposed to delivery adapters.

## Responsibilities and boundaries

- A usecase coordinates domain rules and repository contracts without importing delivery or infrastructure.
- A small workflow uses `internal/usecase/<feature>`; related workflows may use subpackages and share types in their parent package.
- Usecases return application errors that delivery can map without exposing infrastructure details.

## Extending

- Define the usecase interface and implementation in the feature package, with explicit constructor dependencies.
- Keep shared business invariants in domain types and transport validation in delivery.
- Add repository operations shaped around the workflow instead of chaining low-level storage calls.
