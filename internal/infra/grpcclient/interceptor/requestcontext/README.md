# internal/infra/grpcclient/interceptor/requestcontext

## Pattern used

- Applies a default deadline only when the caller has not supplied a shorter one.
- Propagates an existing request ID only when app composition explicitly supplies a metadata key.

## How to extend

- Opt into the downstream service's correlation convention rather than assuming a fixed metadata key.
- Add bounded, transport-neutral context fields here; keep authentication credentials in a dedicated interceptor.
- Do not add retries here because retry safety depends on the called method.
