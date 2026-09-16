# internal/infra/config

## Pattern used

- Shared runtime settings and each transport profile are loaded and validated independently.
- App composition combines `RuntimeConfig` with only its own HTTP or gRPC configuration.

## How to extend

- Put process-neutral settings in `runtime_config.go`; put transport-owned settings in the matching profile file.
- Keep profile tests beside their loader so removing a profile removes its configuration and tests together.
