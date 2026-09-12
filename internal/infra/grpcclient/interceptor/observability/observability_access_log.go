package observability

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/eannchen/go-backend-architecture/internal/logger"
	appobservability "github.com/eannchen/go-backend-architecture/internal/observability"
)

// AccessLog records one structured completion event per client RPC.
type AccessLog struct {
	log    logger.Logger
	policy LogPolicy
}

// LogOutcome exposes transport facts without assigning business meaning to them.
type LogOutcome struct {
	DependencyName string
	FullMethod     string
	Status         codes.Code
	Duration       time.Duration
	Err            error
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
		DependencyName: outcome.rpc.dependency,
		FullMethod:     outcome.rpc.fullMethod,
		Status:         outcome.status,
		Duration:       outcome.duration,
		Err:            outcome.err,
	})
	if !enabled {
		return
	}
	fields := logger.FromPairs(
		keyRPCSystem, "grpc",
		keyRPCService, outcome.rpc.service,
		keyRPCMethod, outcome.rpc.method,
		keyRPCType, outcome.rpc.rpcType,
		keyGRPCStatusCode, int(outcome.status),
		keyDurationMS, outcome.duration.Milliseconds(),
	)
	if outcome.rpc.dependency != "" {
		fields[keyDependencyName] = outcome.rpc.dependency
	}
	if outcome.err != nil {
		fields[keyError] = outcome.err.Error()
		fields[keyErrorChain] = appobservability.ErrorCauseChain(outcome.err)
		fields[keyErrorMessage] = outcome.message
	}
	switch severity {
	case logger.SeverityDebug:
		l.log.Debug(ctx, "gRPC client request completed", fields)
	case logger.SeverityWarn:
		l.log.Warn(ctx, "gRPC client request completed", fields)
	case logger.SeverityError:
		l.log.ErrorNoStack(ctx, "gRPC client request completed", outcome.err, fields)
	default:
		l.log.Info(ctx, "gRPC client request completed", fields)
	}
}
