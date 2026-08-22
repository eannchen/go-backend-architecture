# cmd/grpcapi

## Pattern used

- Owns the standalone gRPC process boot lifecycle.
- Delegates service construction and server lifecycle to `internal/app/grpcapi`.

## How to extend

- Keep signal handling and process exit behavior here.
- Add services and interceptors through the gRPC composition package.
