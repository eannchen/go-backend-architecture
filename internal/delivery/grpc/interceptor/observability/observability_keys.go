package observability

const (
	keyRPCSystem         = "rpc.system"
	keyRPCService        = "rpc.service"
	keyRPCMethod         = "rpc.method"
	keyRPCType           = "rpc.type"
	keyGRPCStatusCode    = "rpc.grpc.status_code"
	keyDurationMS        = "duration.ms"
	keyError             = "error"
	keyErrorChain        = "error.chain"
	keyErrorDetails      = "error.details"
	keyTransportCode     = "error.transport.code"
	keyTransportMessage  = "error.transport.message"
	instrumentationScope = "grpc"
)
