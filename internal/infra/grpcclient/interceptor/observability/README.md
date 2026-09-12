# internal/infra/grpcclient/interceptor/observability

## Pattern used

- One optional unary/stream interceptor coordinates a shared outcome and stream lifecycle while tracing, metrics, and completion logging are selected independently.
- Local client spans are created only when tracing is selected; trace context is injected into outgoing metadata only when the dependency is trusted to share that contract.
- Metrics are opt-in because spans may be sufficient for low-value dependencies, while important dependencies benefit from unsampled aggregate latency and failure measurements.
- Completion logging is opt-in and requires a dependency policy that can select the level or skip an outcome; the shared package does not classify gRPC status codes globally.
- A configured dependency name distinguishes otherwise identical RPC methods called on different remote systems.
- Stream completion is recorded when send or receive terminates; successful `io.EOF` is normalized to an OK outcome.
- Client errors contain only remote transport information because server-side original causes do not cross the wire.

## How to extend

- Keep detailed remote errors in traces and logs while restricting metrics to dependency, service, method, RPC type, and status.
- Wrap streams when adding lifecycle behavior; do not finish a stream span when the stream is merely created.
- Use the interceptor for transport facts. Put typed response interpretation, business metrics, and business logging in the dependency's outbound adapter.
