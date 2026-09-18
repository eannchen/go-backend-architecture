# gRPC request context

This interceptor applies unary-call deadlines and optional request-ID interoperability.

## Responsibilities and boundaries

- Accepts and returns request-ID metadata only when keys are configured; missing IDs are not generated.
- Ignores malformed optional IDs by default, with strict rejection available through configuration.
- Applies a unary server timeout without extending a shorter client deadline.
- Carries metadata into streams without imposing one deadline on the entire stream.
- Passes handler errors through unchanged so services remain the response-mapping boundary.

## Extending

- Add a context value only when every gRPC service needs it.
- Keep authentication, authorization, rate limiting, and error normalization in separate interceptors or services.
