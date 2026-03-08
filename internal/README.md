# internal

Private application code for this backend architecture template.

## Pattern used

- Keep dependencies pointing inward: `delivery -> usecase -> repository contracts`.
- `infra` implements contracts; inner layers never import infra packages.
- Use constructor injection in `internal/app` only (composition root).
- Prefer small interfaces owned by consumers (SOLID interface segregation).

## How to extend

1. Add usecase package in `internal/usecase/<feature>`.
2. Add contracts in `internal/repository` if persistence is needed.
3. Add infra implementation in `internal/infra/...`.
4. Add transport handler in `internal/delivery/...`.
5. Wire everything in `internal/app/wiring.go`.
