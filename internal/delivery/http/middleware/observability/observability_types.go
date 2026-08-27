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
	route  string
	path   string
	header http.Header
}

func newRequestInfo(c *echo.Context) requestInfo {
	request := c.Request()
	route := c.Path()
	if route == "" {
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
