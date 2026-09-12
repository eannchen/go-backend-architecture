# internal/infra/security/tlsconfig

## Pattern used

- Server and client certificates, trust roots, and optional mutual-TLS identities are loaded into standard-library `tls.Config` values.
- TLS 1.2 is the minimum; a configured client CA verifies optional certificates or becomes mandatory when mTLS is enabled.

## How to extend

- Add certificate reload behind a dedicated provider rather than adding file watching to transport servers.
- Keep protocol-specific credential wrappers in the relevant app composition package.
