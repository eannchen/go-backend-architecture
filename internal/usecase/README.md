# internal/usecase

Application orchestration layer (usecase-centric).

## Pattern used

- Package-level interface + private `impl` for testability and encapsulation.
- Usecase receives dependencies through constructors.
- Return typed app errors (`internal/apperr`) for stable handler mapping.

## How to extend

- Create `internal/usecase/<feature>/`.
- Expose `Usecase` interface and `New(...) Usecase`.
- Keep framework and SQL details out of this layer.
