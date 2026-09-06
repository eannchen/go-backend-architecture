# internal/infra/security/tlsconfig

## Pattern used

- Server certificates and optional client trust roots are loaded into a standard-library `tls.Config`.
- TLS 1.2 is the minimum; a configured client CA verifies optional certificates or becomes mandatory when mTLS is enabled.

## How to extend

- Add certificate reload behind a dedicated provider rather than adding file watching to transport servers.
- Keep protocol-specific credential wrappers in the relevant app composition package.
