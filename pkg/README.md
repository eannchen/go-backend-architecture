# Public packages

This directory is reserved for packages intentionally imported by other Go modules.

## Boundaries

- New code belongs under `internal` unless external reuse is a supported requirement.
- Public packages must not expose application-specific wiring or internal infrastructure types.

## Extending

- Add a package here only after defining its external API and compatibility expectations.
