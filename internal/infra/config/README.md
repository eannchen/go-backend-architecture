# Runtime configuration

This package loads and validates environment-backed configuration before application startup.

## Responsibilities and boundaries

- `LoadRuntime` reads shared settings; `LoadHTTPAPI`, `LoadGRPCAPI`, and `LoadKafkaWorker` add the settings for their executable.
- Configuration values are normalized and validated once, then injected into constructors.
- Profile-specific configuration and tests remain in separate files so the profile selector can remove unused pairs.

## Extending

- Put settings shared by executable types in `runtime_config.go` and transport-specific settings in the matching profile file.
- Add defaults, normalization, validation, and tests with each new setting.
- Do not read environment variables from delivery, usecase, or domain packages.
