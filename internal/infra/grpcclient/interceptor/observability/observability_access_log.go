package observability

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/eannchen/go-backend-architecture/internal/logger"
)

// AccessLog records one structured completion event per client RPC.
type AccessLog struct {
	log    logger.Logger
	policy LogPolicy
}

// LogOutcome exposes transport facts without assigning business meaning to them.
type LogOutcome struct {
	// DependencyName is the optional stable application name configured for the remote service.
	DependencyName string
	// RPCMethod is the normalized fully-qualified gRPC method, such as package.Service/Method.
	RPCMethod string
	// GRPCStatusCode is the native status returned by the remote call.
	GRPCStatusCode codes.Code
	// Duration covers the full logical call, including a stream's lifetime.
	Duration time.Duration
	// RPCError is the exact error returned by gRPC, or nil for an OK status.
	RPCError error
}

// LogPolicy decides whether and at what level one dependency logs an outcome.
type LogPolicy func(LogOutcome) (logger.Severity, bool)

// NewAccessLog creates client completion logging governed by dependency policy.
func NewAccessLog(log logger.Logger, policy LogPolicy) *AccessLog {
	if log == nil {
		log = logger.NoopLogger{}
	}
	return &AccessLog{log: log, policy: policy}
}

// Record delegates severity to the dependency policy rather than interpreting status codes globally.
func (l *AccessLog) Record(ctx context.Context, outcome rpcOutcome) {
	if l.policy == nil {
		return
	}
	severity, enabled := l.policy(LogOutcome{
		DependencyName: outcome.rpc.dependencyName,
		RPCMethod:      outcome.rpc.method,
		GRPCStatusCode: outcome.responseStatusCode,
		Duration:       outcome.duration,
		RPCError:       outcome.callError,
	})
	if !enabled {
		return
	}
	fields := logger.FromPairs(
		keyRPCSystemName, "grpc",
		keyRPCMethod, outcome.rpc.method,
		keyApplicationRPCCallType, outcome.rpc.callType,
		keyRPCResponseStatusCode, outcome.responseStatusName(),
		keyLogDurationMS, outcome.duration.Milliseconds(),
	)
	if outcome.rpc.serverAddress != "" {
		fields[keyServerAddress] = outcome.rpc.serverAddress
	}
	if outcome.rpc.serverPort > 0 {
		fields[keyServerPort] = outcome.rpc.serverPort
	}
	if outcome.rpc.dependencyName != "" {
		fields[keyApplicationDependencyName] = outcome.rpc.dependencyName
	}
	if errorType := outcome.errorType(); errorType != "" {
		fields[keyErrorType] = errorType
		fields[keyApplicationRPCStatusMessage] = outcome.responseStatusMessage
	}
	switch severity {
	case logger.SeverityDebug:
		l.log.Debug(ctx, "gRPC client request completed", fields)
	case logger.SeverityWarn:
		l.log.Warn(ctx, "gRPC client request completed", fields)
	case logger.SeverityError:
		l.log.ErrorNoStack(ctx, "gRPC client request completed", outcome.callError, fields)
	default:
		l.log.Info(ctx, "gRPC client request completed", fields)
	}
}
