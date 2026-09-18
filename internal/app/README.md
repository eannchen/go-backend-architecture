# Application composition

This directory contains the composition roots for executable processes.

## Responsibilities and boundaries

- `runtime` owns infrastructure and lifecycle code shared by executable types.
- `httpapi` and `grpcapi` choose concrete implementations and assemble their delivery adapters.
- Composition packages contain wiring and lifecycle orchestration, not business rules or protocol mapping.

## Extending

- Add a sibling package for a new executable process.
- Put a dependency in `runtime` only when multiple process types own it in the same way.
- Keep process-specific constructors and shutdown order in the corresponding composition package.
