# internal/delivery/http/binding

## Pattern used

- `NormalizeBinder` wraps any `echo.Binder`: delegates bind, then trims and optionally case-converts string fields via struct tags (`trim:"false"`, `case:"lower"`, `case:"upper"`).
- Handles nested structs, pointers, and slices.
- Injected into `NewServer` via constructor; pass `nil` for the default normalize binder.

## How to extend

- New normalization: add a struct tag, read it in `normalizeStruct`, apply in a helper. Add tests.
- Custom binder: implement `echo.Binder`, pass it to `NewServer` in wiring.
- Opt-out per field: `trim:"false"`.
