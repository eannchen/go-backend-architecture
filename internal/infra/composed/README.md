# Composed infrastructure adapters

This directory combines multiple adapters behind one repository contract.

## Responsibilities and boundaries

- A composed adapter coordinates multiple adapters, such as a cache lookup, primary-store read, cache population, and invalidation.
- It implements the same contract as the primary adapter, keeping usecases unaware of composition.
- Cache consistency and dependency-failure policy are explicit in the composed implementation.

## Extending

- Add one feature package when a repository operation requires multiple adapters.
- Inject the primary and supporting adapters through the constructor.
- Document and test the selected cache or coordination pattern, acceptable staleness, and failure behavior.
