package httpcontext

import (
	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
)

const keyErrorOutcome = "httpcontext.error_outcome"

type Details = apperr.Details

// ErrorOutcome is the responder-normalized failure observed after a request completes.
type ErrorOutcome struct {
	// OriginalError is the internal error used for logging and span recording.
	OriginalError error
	// ApplicationErrorCode is the non-HTTP code assigned by the application or delivery layer.
	ApplicationErrorCode string
	// ApplicationErrorMessage is the safe message associated with ApplicationErrorCode.
	ApplicationErrorMessage string
	// DiagnosticDetails are optional trace and log fields; they are not written to responses or metrics.
	DiagnosticDetails Details
}

// SetErrorOutcome records the complete normalized failure for downstream middleware.
func SetErrorOutcome(c *echo.Context, outcome ErrorOutcome) {
	c.Set(keyErrorOutcome, outcome)
}

// ErrorOutcomeFrom returns the normalized failure recorded for this request.
func ErrorOutcomeFrom(c *echo.Context) (ErrorOutcome, bool) {
	outcome, ok := c.Get(keyErrorOutcome).(ErrorOutcome)
	return outcome, ok
}
