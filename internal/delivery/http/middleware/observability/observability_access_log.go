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
		keyHTTPRequestMethod, outcome.request.requestMethod,
		keyURLPath, outcome.request.urlPath,
		keyURLScheme, outcome.request.urlScheme,
		keyHTTPResponseStatusCode, outcome.responseStatusCode,
		keyLogDurationMS, outcome.duration.Milliseconds(),
	)
	if outcome.request.requestMethodOriginal != "" {
		fields[keyHTTPRequestMethodOriginal] = outcome.request.requestMethodOriginal
	}
	if outcome.request.routeTemplate != "" {
		fields[keyHTTPRoute] = outcome.request.routeTemplate
	}
	if errorType := outcome.errorType(); errorType != "" {
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
	if outcome.responseStatusCode >= 500 {
		l.log.ErrorNoStack(ctx, "request completed", outcome.applicationError.originalError, fields)
		return
	}
	l.log.Info(ctx, "request completed", fields)
}
