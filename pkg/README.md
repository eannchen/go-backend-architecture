# Public packages

This directory contains technical libraries intended for reuse by other Go modules.

## Boundaries

- New code belongs under `internal` unless external reuse is a supported requirement.
- Public packages own their typed configuration and import no project `internal` packages. Environment loading and application wiring stay internal.

## Extending

- Add a package here only after defining its external API and compatibility expectations.
