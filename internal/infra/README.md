# internal/infra

Infrastructure implementations. Implements repository and other inner-layer contracts; app wires implementations and never exposes vendor types to usecase or delivery.

## Pattern used

- Adapter pattern: each subpackage implements one kind of contract (persistence, cache, logging, observability, config).
- Package boundaries are per concern; vendor and SDK details stay inside infra.
- Only contract-friendly types and interfaces are used by outer layers.
- Infra returns standard Go errors (`fmt.Errorf` with `%w`). Usecases are responsible for wrapping infra errors into application errors (`apperr`).

## How to extend

- Add a new subpackage per infrastructure concern; implement the corresponding contract from repository or the dedicated contract package.
- Keep constructors explicit and injectable; wire in app only.
- Do not expose vendor or SDK types outside this tree.
