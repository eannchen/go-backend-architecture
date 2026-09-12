package observability

const (
	// keyRPCSystemName is the OTel RPC-system attribute; gRPC requires the value "grpc".
	keyRPCSystemName = "rpc.system.name"
	// keyRPCMethod is the OTel fully-qualified logical method, such as package.Service/Method.
	keyRPCMethod = "rpc.method"
	// keyRPCResponseStatusCode is the OTel native gRPC status name, such as OK or UNAVAILABLE.
	keyRPCResponseStatusCode = "rpc.response.status_code"
	// keyServerAddress is the OTel logical destination derived from the configured gRPC target.
	keyServerAddress = "server.address"
	// keyServerPort is the OTel logical destination port when it can be parsed safely.
	keyServerPort = "server.port"
	// keyErrorType is the stable OTel failure class and uses the native gRPC status name.
	keyErrorType = "error.type"
	// keyLogDurationMS is a template log field; span duration and metric histograms record time separately.
	keyLogDurationMS = "duration.ms"
	// keyApplicationDependencyName is the application's stable alias for a remote dependency,
	// such as "billing-service". OTel defines server.address for the network destination but no
	// equivalent logical dependency name, so this template-owned field stays under app.*.
	keyApplicationDependencyName = "app.dependency.name"
	// keyApplicationRPCCallType distinguishes unary, client-streaming, server-streaming, and
	// bidirectional-streaming calls. OTel's RPC conventions do not define this call-shape field,
	// so app.* makes clear that it is our convention rather than a standard rpc.* attribute.
	keyApplicationRPCCallType = "app.rpc.call_type"
	// keyApplicationRPCStatusMessage stores status.Convert(err).Message() from the remote response.
	// Message() is a standard gRPC status value, but OTel defines no standard attribute for it;
	// app.* therefore marks the attribute name as ours. Keep this potentially sensitive,
	// high-cardinality text in individual logs and spans, never metric dimensions.
	keyApplicationRPCStatusMessage = "app.rpc.status_message"
	// instrumentationScope identifies spans produced by this gRPC client interceptor.
	instrumentationScope = "grpc-client"
)
