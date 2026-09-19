package observabilitymw

import (
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/delivery/http/httpcontext"
	"github.com/eannchen/go-backend-architecture/internal/observability"
)

type requestInfo struct {
	// requestMethod is the normalized native HTTP method used by OTel and metrics.
	requestMethod string
	// requestMethodOriginal retains an unknown method when requestMethod is the bounded _OTHER value.
	requestMethodOriginal string
	// routeTemplate is the registered template (for example, /users/:id). Its bounded
	// values are suitable for grouping traces, logs, and metrics.
	routeTemplate string
	// urlPath is the concrete request path (for example, /users/42). It helps
	// diagnose individual requests but is intentionally excluded from metrics.
	urlPath string
	// urlScheme reflects the immediate TLS connection without trusting forwarded headers.
	urlScheme string
	// propagationHeaders carry traceparent and tracestate into the tracing adapter.
	propagationHeaders http.Header
}

func newRequestInfo(c *echo.Context) requestInfo {
	request := c.Request()
	routeTemplate := c.Path()
	requestMethod, requestMethodOriginal := normalizeRequestMethod(request.Method)
	urlScheme := "http"
	if request.TLS != nil {
		urlScheme = "https"
	}
	return requestInfo{
		requestMethod:         requestMethod,
		requestMethodOriginal: requestMethodOriginal,
		routeTemplate:         routeTemplate,
		urlPath:               request.URL.Path,
		urlScheme:             urlScheme,
		propagationHeaders:    request.Header,
	}
}

func (i requestInfo) spanStartFields() observability.Fields {
	fields := observability.FromPairs(
		keyHTTPRequestMethod, i.requestMethod,
		keyURLPath, i.urlPath,
		keyURLScheme, i.urlScheme,
	)
	if i.requestMethodOriginal != "" {
		fields[keyHTTPRequestMethodOriginal] = i.requestMethodOriginal
	}
	// OTel forbids substituting the concrete path when the router did not match.
	if i.routeTemplate != "" {
		fields[keyHTTPRoute] = i.routeTemplate
	}
	return fields
}

func (i requestInfo) spanName() string {
	method := i.requestMethod
	if method == "_OTHER" {
		method = "HTTP"
	}
	if i.routeTemplate == "" {
		return method
	}
	return method + " " + i.routeTemplate
}

func normalizeRequestMethod(method string) (string, string) {
	switch method {
	case http.MethodConnect,
		http.MethodDelete,
		http.MethodGet,
		http.MethodHead,
		http.MethodOptions,
		http.MethodPatch,
		http.MethodPost,
		http.MethodPut,
		"QUERY",
		http.MethodTrace:
		return method, ""
	default:
		return "_OTHER", method
	}
}

type requestOutcome struct {
	// request contains immutable transport facts captured before the handler runs.
	request requestInfo
	// duration measures handler execution and is used by logs and aggregate metrics.
	duration time.Duration
	// responseStatusCode is the native HTTP status resolved from Echo's response and handler error.
	responseStatusCode int
	// applicationError contains responder-owned application error metadata, when available.
	applicationError applicationErrorInfo
}

func newRequestOutcome(c *echo.Context, request requestInfo, duration time.Duration, handlerErr error) requestOutcome {
	_, responseStatusCode := echo.ResolveResponseStatus(c.Response(), handlerErr)
	return requestOutcome{
		request:            request,
		duration:           duration,
		responseStatusCode: responseStatusCode,
		applicationError:   inspectApplicationError(c),
	}
}

type applicationErrorInfo struct {
	// originalError is the internal Go error retained for logging and span error recording.
	originalError error
	// causeChain is the diagnostic unwrap chain and may contain high-cardinality internal text.
	causeChain string
	// diagnosticDetails contains serialized responder details for traces and logs only.
	diagnosticDetails string
	// applicationErrorCode is the non-HTTP code assigned by the application or delivery layer.
	applicationErrorCode string
	// applicationErrorMessage is the safe message associated with applicationErrorCode.
	applicationErrorMessage string
}

func inspectApplicationError(c *echo.Context) applicationErrorInfo {
	outcome, ok := httpcontext.ErrorOutcomeFrom(c)
	if !ok {
		return applicationErrorInfo{}
	}
	return applicationErrorInfo{
		originalError:           outcome.OriginalError,
		causeChain:              observability.ErrorCauseChain(outcome.OriginalError),
		diagnosticDetails:       outcome.DiagnosticDetails.String(),
		applicationErrorCode:    outcome.ApplicationErrorCode,
		applicationErrorMessage: outcome.ApplicationErrorMessage,
	}
}

// errorType follows the OTel HTTP server rule: a 5xx response means the server
// operation failed, while a 4xx response normally represents a client failure
// and must not mark the server span as failed. OTel stores the status as a string.
func (o requestOutcome) errorType() string {
	if o.responseStatusCode < http.StatusInternalServerError {
		return ""
	}
	return strconv.Itoa(o.responseStatusCode)
}
