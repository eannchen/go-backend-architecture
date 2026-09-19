# TLS configuration

This package loads certificates and trust roots into standard-library `tls.Config` values without importing HTTP or gRPC.

## Responsibilities and boundaries

- Builds separate client and server configurations with TLS 1.2 as the minimum.
- Server trust roots can verify optional client certificates or require mutual TLS (mTLS), according to configuration.
- Returns standard-library configuration without importing HTTP or gRPC credential types.

## Extending

- Add certificate reload through a dedicated provider rather than file watching inside a transport server.
- Keep client and server identity fields explicit when extending configuration.
- Wrap returned configurations with protocol credentials in app composition.
