# internal/usecase

## Pattern used

- Interface + private impl per usecase for testability. Both in the same `<feature>_usecase.go` file.
- Dependencies via constructors. Returns `apperr` errors.
- Single-capability feature: `<feature>/<feature>_usecase.go` (e.g. `health/health_usecase.go`).
- Multi-capability feature: subdirs per capability (e.g. `auth/otp/otp_usecase.go`); shared types in parent (`auth/auth_types.go`).

## How to extend

- Create `<feature>/<feature>_usecase.go` with interface + `New(...)`.
- For multi-capability features, add subdirs with one `<capability>_usecase.go` each; keep shared types in parent.
- Keep framework and SQL details out.
