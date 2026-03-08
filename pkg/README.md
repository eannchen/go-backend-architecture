# pkg

Public reusable packages (optional).

## Pattern used

- Host only APIs intentionally exported for reuse outside this module.
- Keep package surface stable and version-friendly.

## How to extend

- Prefer `internal` by default; move to `pkg` only when external reuse is required.
- Document compatibility expectations before adding new public packages.
