# internal/delivery

## Pattern used

- HTTP handlers and gRPC services depend on usecase interfaces, not concrete implementations.
- Transport DTOs stay in delivery; usecase models stay in usecase.
- Transport adapters map protocol-specific requests, responses, and errors at the boundary.

## How to extend

- Add feature handlers under `http/handler/<feature>/`.
- Register routes through `RouteRegistrar`.
- Keep HTTP-specific validation and response mapping here.
- Add gRPC service adapters under `grpc/service/<feature>/` and keep generated protobuf types inside delivery.
