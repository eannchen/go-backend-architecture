# gRPC TLS and mTLS

Local development uses plaintext unless `GRPC_TLS_ENABLED` is set to `true`.

## TLS server

Configure a PEM certificate chain and matching private key:

```dotenv
GRPC_TLS_ENABLED=true
GRPC_TLS_CERT_FILE=certs/server.pem
GRPC_TLS_KEY_FILE=certs/server-key.pem
GRPC_TLS_CLIENT_CA_FILE=
GRPC_TLS_REQUIRE_CLIENT_CERT=false
```

Call the server with a trusted root certificate:

```bash
grpcurl -cacert certs/ca.pem localhost:9090 list
grpcurl -cacert certs/ca.pem -d '{}' localhost:9090 grpc.health.v1.Health/Check
```

For local plaintext mode, use `grpcurl -plaintext` instead of `-cacert`.

## Mutual TLS

Configure the CA allowed to issue client certificates:

```dotenv
GRPC_TLS_ENABLED=true
GRPC_TLS_CERT_FILE=certs/server.pem
GRPC_TLS_KEY_FILE=certs/server-key.pem
GRPC_TLS_CLIENT_CA_FILE=certs/client-ca.pem
GRPC_TLS_REQUIRE_CLIENT_CERT=true
```

The client must trust the server CA and present its own certificate and key:

```bash
grpcurl \
  -cacert certs/ca.pem \
  -cert certs/client.pem \
  -key certs/client-key.pem \
  -d '{}' \
  localhost:9090 \
  grpc.health.v1.Health/Check
```

Reflection must also be enabled for `grpcurl` to discover services without local protobuf descriptors. Keep reflection disabled when callers do not need it.

Do not commit private keys. Production certificate issuance, mounting, rotation, and file permissions belong to the deployment environment.
