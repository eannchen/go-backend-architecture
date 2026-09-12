package observability

const (
	// keyRPCSystemName is the OTel RPC-system attribute; gRPC requires the value "grpc".
	keyRPCSystemName = "rpc.system.name"
	// keyRPCMethod is the OTel fully-qualified logical method, such as package.Service/Method.
	keyRPCMethod = "rpc.method"
	// keyRPCResponseStatusCode is the OTel native gRPC status name, such as OK or UNAVAILABLE.
	keyRPCResponseStatusCode = "rpc.response.status_code"
	// keyErrorType is the stable OTel failure class and uses the native gRPC status name.
	keyErrorType = "error.type"
	// keyLogDurationMS is a template log field; span duration and metric histograms record time separately.
	keyLogDurationMS = "duration.ms"
	// keyApplicationRPCCallType is a template field distinguishing unary and streaming call shapes.
	keyApplicationRPCCallType = "app.rpc.call_type"
	// keyApplicationErrorCauseChain is a template diagnostic field containing the internal Go error chain.
	keyApplicationErrorCauseChain = "app.error.cause_chain"
	// keyApplicationErrorDetails is a template field containing safe serialized responder details.
	keyApplicationErrorDetails = "app.error.details"
	// keyApplicationErrorCode is a template field containing the code assigned by apperr.
	keyApplicationErrorCode = "app.error.code"
	// keyApplicationErrorMessage is a template field containing the safe message returned to the client.
	keyApplicationErrorMessage = "app.error.message"
	// instrumentationScope identifies spans produced by this gRPC server interceptor.
	instrumentationScope = "grpc-server"
)
