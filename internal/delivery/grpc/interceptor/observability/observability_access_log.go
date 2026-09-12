package observability

import (
	"context"

	"github.com/eannchen/go-backend-architecture/internal/logger"
)

// AccessLog writes one structured completion log per RPC.
type AccessLog struct {
	log logger.Logger
}

// NewAccessLog creates gRPC request access logging.
func NewAccessLog(log logger.Logger) *AccessLog {
	if log == nil {
		log = logger.NoopLogger{}
	}
	return &AccessLog{log: log}
}

// Record relies on the logger's context provider for request and trace IDs.
func (l *AccessLog) Record(ctx context.Context, outcome rpcOutcome) {
	errorType := outcome.errorType()
	fields := logger.FromPairs(
		keyRPCSystemName, "grpc",
		keyRPCMethod, outcome.rpc.method,
		keyApplicationRPCCallType, outcome.rpc.callType,
		keyRPCResponseStatusCode, outcome.responseStatusName(),
		keyLogDurationMS, outcome.duration.Milliseconds(),
	)
	if errorType != "" {
		fields[keyErrorType] = errorType
	}
	if outcome.applicationError.causeChain != "" {
		fields[keyApplicationErrorCauseChain] = outcome.applicationError.causeChain
	}
	if outcome.applicationError.diagnosticDetails != "" {
		fields[keyApplicationErrorDetails] = outcome.applicationError.diagnosticDetails
	}
	if outcome.applicationError.applicationErrorCode != "" {
		fields[keyApplicationErrorCode] = outcome.applicationError.applicationErrorCode
	}
	if outcome.applicationError.applicationErrorMessage != "" {
		fields[keyApplicationErrorMessage] = outcome.applicationError.applicationErrorMessage
	}
	if errorType != "" {
		l.log.ErrorNoStack(ctx, "request completed", outcome.applicationError.originalError, fields)
		return
	}
	l.log.Info(ctx, "request completed", fields)
}
