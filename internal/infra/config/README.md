# Runtime configuration

This package loads and validates environment-backed configuration before application startup.

## Responsibilities and boundaries

- `LoadRuntime` reads shared settings; each executable has a loader for its own settings.
- Loaders normalize and validate environment values before injection. Client constructors also validate configuration supplied directly in Go.
- Profile-specific configuration and tests remain in separate files so the profile selector can remove unused pairs.

## Extending

- Put settings shared by executable types in `runtime_config.go` and transport-specific settings in the matching profile file.
- Add defaults, normalization, validation, and tests with each new setting.
- Do not read environment variables from delivery, usecase, or domain packages.
