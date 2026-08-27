package observabilitymw

import (
	"context"

	"github.com/eannchen/go-backend-architecture/internal/logger"
)

// AccessLog writes one structured completion log per HTTP request.
type AccessLog struct {
	log logger.Logger
}

// NewAccessLog creates HTTP request access logging.
func NewAccessLog(log logger.Logger) *AccessLog {
	if log == nil {
		log = logger.NoopLogger{}
	}
	return &AccessLog{log: log}
}

// Record relies on the logger's context provider for request and trace IDs.
func (l *AccessLog) Record(ctx context.Context, outcome requestOutcome) {
	fields := logger.FromPairs(
		keyHTTPRequestMethod, outcome.request.method,
		keyHTTPRoute, outcome.request.route,
		keyURLPath, outcome.request.path,
		keyHTTPResponseStatus, outcome.status,
		keyDurationMS, outcome.duration.Milliseconds(),
	)
	if outcome.errorInfo.original != nil {
		fields[keyError] = outcome.errorInfo.original.Error()
		fields[keyErrorChain] = outcome.errorInfo.chain
	}
	if outcome.errorInfo.details != "" {
		fields[keyErrorDetails] = outcome.errorInfo.details
	}
	if outcome.errorInfo.code != "" {
		fields[keyErrorCode] = outcome.errorInfo.code
	}
	if outcome.errorInfo.message != "" {
		fields[keyErrorMessage] = outcome.errorInfo.message
	}
	if outcome.status >= 500 {
		l.log.ErrorNoStack(ctx, "request completed", outcome.errorInfo.original, fields)
		return
	}
	l.log.Info(ctx, "request completed", fields)
}
