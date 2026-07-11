# internal/util/testutil

## Pattern used

- Test-only helpers keep integration setup consistent without entering production code.
- Explicit shell variables take precedence over values in a local `.env` file.

## How to extend

- Keep helpers deterministic and scoped to the calling test.
- Do not put production configuration loading in this package.
