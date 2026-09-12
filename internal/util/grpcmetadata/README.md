# internal/util/grpcmetadata

## Pattern used

- Normalizes and validates configurable gRPC metadata keys before transports start.
- Keeps shared metadata syntax rules independent of server and client policy.

## How to extend

- Add transport-level syntax helpers only; keep request-ID, authentication, and propagation policy in their owning interceptors.
