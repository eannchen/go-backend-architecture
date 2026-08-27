package observabilitymw

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/delivery/http/httpcontext"
	"github.com/eannchen/go-backend-architecture/internal/observability"
)

type requestInfo struct {
	method string
	// route is the registered template (for example, /users/:id). Its bounded
	// values are suitable for grouping traces, logs, and metrics.
	route string
	// path is the concrete request path (for example, /users/42). It helps
	// diagnose individual requests but is intentionally excluded from metrics.
	path   string
	header http.Header
}

func newRequestInfo(c *echo.Context) requestInfo {
	request := c.Request()
	route := c.Path()
	if route == "" {
		// Keep unmatched requests in one bounded group while preserving the
		// concrete path separately for tracing and access logs.
		route = "unmatched"
	}
	return requestInfo{
		method: request.Method,
		route:  route,
		path:   request.URL.Path,
		header: request.Header,
	}
}

func (i requestInfo) fields() observability.Fields {
	return observability.FromPairs(
		keyHTTPRequestMethod, i.method,
		keyHTTPRoute, i.route,
	)
}

type requestOutcome struct {
	request   requestInfo
	duration  time.Duration
	status    int
	errorInfo requestErrorInfo
}

func newRequestOutcome(c *echo.Context, request requestInfo, duration time.Duration, handlerErr error) requestOutcome {
	_, status := echo.ResolveResponseStatus(c.Response(), handlerErr)
	return requestOutcome{
		request:   request,
		duration:  duration,
		status:    status,
		errorInfo: inspectRequestError(c),
	}
}

type requestErrorInfo struct {
	original error
	chain    string
	details  string
	code     string
	message  string
}

func inspectRequestError(c *echo.Context) requestErrorInfo {
	original := httpcontext.Error(c)
	details := httpcontext.ErrorDetails(c)
	code, message := httpcontext.TransportError(c)
	return requestErrorInfo{
		original: original,
		chain:    observability.ErrorCauseChain(original),
		details:  details.String(),
		code:     code,
		message:  message,
	}
}
