# internal/delivery

## Pattern used

- Handler depends on usecase interface, not concrete impl.
- Transport DTOs stay in delivery; usecase models stay in usecase.
- Middleware handles cross-cutting concerns (trace, request-id, timeout).

## How to extend

- Add feature handlers under `http/handler/<feature>/`.
- Register routes through `RouteRegistrar`.
- Keep HTTP-specific validation and response mapping here.
