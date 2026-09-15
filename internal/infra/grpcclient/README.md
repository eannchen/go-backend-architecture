# internal/infra/grpcclient

## Pattern used

- Owns only reusable outbound gRPC connection mechanics: the target, transport credentials, message limits, lifecycle, and explicitly supplied interceptors.
- Transport credentials are built by app composition and must be explicit, including deliberate plaintext credentials for local development.
- Connections are lazy; availability is established by bounded RPC calls rather than blocking process startup.
- It does not assume that every dependency shares deadlines, trace context, request IDs, authentication, metrics, or logging policy.

## How to extend

- Add one service-specific adapter under `internal/infra/external/` for each remote dependency; let that adapter implement the contract owned by `internal/repository/external/`.
- Select connection-wide interceptors in that adapter's app wiring. Internal dependencies may share trace and request context; third-party dependencies should receive only metadata required by their contract.
- Keep method-specific deadlines, retries, error mapping, and business-oriented telemetry in the adapter or a narrow generated-client wrapper.
