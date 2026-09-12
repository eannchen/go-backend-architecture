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
	keyErrorMessage      = "error.message"
	keyDependencyName    = "dependency.name"
	instrumentationScope = "grpc-client"
)
