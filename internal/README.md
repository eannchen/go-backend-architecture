# internal

## Pattern used

- Dependencies point inward: `delivery -> usecase -> repository contracts`.
- `infra` implements contracts; inner layers never import infra.
- Constructor injection wired in `internal/app` (composition root).
- Small, consumer-owned interfaces (ISP).

## How to extend

1. Usecase in `internal/usecase/<feature>/`.
2. Contracts in `internal/repository/<area>/`.
3. Infra implementation in `internal/infra/...`.
4. Transport handler in `internal/delivery/...`.
5. Wire in `internal/app/wiring.go`.
