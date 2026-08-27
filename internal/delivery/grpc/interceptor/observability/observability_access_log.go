package observability

import (
	"context"

	"google.golang.org/grpc/codes"

	"github.com/eannchen/go-backend-architecture/internal/logger"
)

// AccessLog writes one structured completion log per RPC.
type AccessLog struct {
	log logger.Logger
}

func NewAccessLog(log logger.Logger) *AccessLog {
	if log == nil {
		log = logger.NoopLogger{}
	}
	return &AccessLog{log: log}
}

func (l *AccessLog) Record(ctx context.Context, outcome rpcOutcome) {
	fields := logger.FromPairs(
		keyRPCSystem, "grpc",
		keyRPCService, outcome.rpc.service,
		keyRPCMethod, outcome.rpc.method,
		keyRPCType, outcome.rpc.rpcType,
		keyGRPCStatusCode, outcome.code.String(),
		keyDurationMS, outcome.duration.Milliseconds(),
	)
	if outcome.err != nil {
		fields[keyError] = outcome.errorInfo.original.Error()
		fields[keyErrorChain] = outcome.errorInfo.chain
		fields[keyTransportCode] = outcome.errorInfo.transportCode
		fields[keyTransportMessage] = outcome.errorInfo.transportMessage
		if outcome.errorInfo.details != "" {
			fields[keyErrorDetails] = outcome.errorInfo.details
		}
	}
	if isServerError(outcome.code) {
		l.log.ErrorNoStack(ctx, "gRPC request completed", outcome.errorInfo.original, fields)
		return
	}
	l.log.Info(ctx, "gRPC request completed", fields)
}

func isServerError(code codes.Code) bool {
	switch code {
	case codes.Unknown, codes.DeadlineExceeded, codes.Unimplemented, codes.Internal, codes.Unavailable, codes.DataLoss:
		return true
	default:
		return false
	}
}
