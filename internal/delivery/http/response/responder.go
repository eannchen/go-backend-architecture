package response

import (
	"context"
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	"github.com/eannchen/go-backend-architecture/internal/delivery/http/httpcontext"
)

// Code represents a transport-level error code. All handler and middleware error
// types are declared here so the full set is visible in one place.
type Code string

const (
	CodeInvalidQuery     Code = "INVALID_QUERY"
	CodeInvalidRequestID Code = "INVALID_REQUEST_ID"
	CodeRequestCanceled  Code = "REQUEST_CANCELED"

	// 499 is the conventional server-side status for a client that closes the
	// request before a response can be completed.
	statusClientClosedRequest = 499
)

func (c Code) toHTTPStatus() int {
	if status, ok := codeStatusMap[c]; ok {
		return status
	}
	return http.StatusInternalServerError
}

// codeStatusMap maps every known error code to its HTTP status.
// Delivery-level codes and apperr codes share the same lookup.
var codeStatusMap = map[Code]int{
	CodeInvalidQuery:     http.StatusBadRequest,
	CodeInvalidRequestID: http.StatusBadRequest,
	CodeRequestCanceled:  statusClientClosedRequest,

	Code(apperr.CodeInvalidArgument): http.StatusBadRequest,
	Code(apperr.CodeUnauthorized):    http.StatusUnauthorized,
	Code(apperr.CodeForbidden):       http.StatusForbidden,
	Code(apperr.CodeNotFound):        http.StatusNotFound,
	Code(apperr.CodeConflict):        http.StatusConflict,
	Code(apperr.CodeTooManyRequests): http.StatusTooManyRequests,
	Code(apperr.CodeUnavailable):     http.StatusServiceUnavailable,
	Code(apperr.CodeTimeout):         http.StatusGatewayTimeout,
	Code(apperr.CodeInternal):        http.StatusInternalServerError,
}

// Details is forwarded from httpcontext so Responder's public API stays self-contained.
type Details = httpcontext.Details

// Responder writes transport responses and records metadata for observability middleware.
type Responder interface {
	Success(c *echo.Context, status int, payload any) error
	Error(c *echo.Context, err error, code Code, message string, details ...Details) error
	InvalidQuery(c *echo.Context, err error, message string, details ...Details) error
	AppError(c *echo.Context, err error) error
	// AppErrorWithPayload records the app error for observability and responds with payload
	// instead of the standard error body; status is derived from the error like AppError.
	AppErrorWithPayload(c *echo.Context, err error, payload any) error
	// StartSSE opens a Server-Sent Events response after validation succeeds.
	StartSSE(c *echo.Context) (*SSEStream, error)
}

type responder struct{}

// NewResponder creates an injectable HTTP responder.
func NewResponder() Responder {
	return &responder{}
}

type errorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (r *responder) Success(c *echo.Context, status int, payload any) error {
	return c.JSON(status, payload)
}

func (r *responder) Error(c *echo.Context, err error, code Code, message string, details ...Details) error {
	httpcontext.SetError(c, err)
	httpcontext.SetErrorDetails(c, optionalDetails(details...))
	return r.writeError(c, code, message)
}

func (r *responder) InvalidQuery(c *echo.Context, err error, message string, details ...Details) error {
	httpcontext.SetError(c, err)
	httpcontext.SetErrorDetails(c, optionalDetails(details...))
	return r.writeError(c, CodeInvalidQuery, message)
}

func (r *responder) AppError(c *echo.Context, err error) error {
	httpcontext.SetError(c, err)
	if handled, responseErr := r.writeContextError(c, err); handled {
		return responseErr
	}
	appErr, ok := apperr.As(err)
	if !ok {
		return r.writeError(c, Code(apperr.CodeInternal), "internal server error")
	}
	httpcontext.SetErrorDetails(c, appErr.Details)
	return r.writeError(c, Code(appErr.Code), appErr.Message)
}

func (r *responder) writeError(c *echo.Context, code Code, message string) error {
	httpcontext.SetTransportError(c, string(code), message)
	return c.JSON(code.toHTTPStatus(), errorPayload{
		Code:    string(code),
		Message: message,
	})
}

func (r *responder) AppErrorWithPayload(c *echo.Context, err error, payload any) error {
	httpcontext.SetError(c, err)
	if handled, responseErr := r.writeContextError(c, err); handled {
		return responseErr
	}
	appErr, ok := apperr.As(err)
	if !ok {
		return c.JSON(Code(apperr.CodeInternal).toHTTPStatus(), payload)
	}
	httpcontext.SetErrorDetails(c, appErr.Details)
	httpcontext.SetTransportError(c, string(appErr.Code), appErr.Message)
	return c.JSON(Code(appErr.Code).toHTTPStatus(), payload)
}

func (r *responder) writeContextError(c *echo.Context, err error) (bool, error) {
	switch {
	case errors.Is(err, context.Canceled):
		httpcontext.SetTransportError(c, string(CodeRequestCanceled), "request canceled")
		return true, c.NoContent(statusClientClosedRequest)
	case errors.Is(err, context.DeadlineExceeded):
		return true, r.writeError(c, Code(apperr.CodeTimeout), "request timed out")
	default:
		return false, nil
	}
}

func optionalDetails(details ...Details) Details {
	if len(details) == 0 {
		return nil
	}
	return details[0]
}

func Fields(pairs ...any) Details {
	if len(pairs) == 0 {
		return nil
	}

	details := make(Details, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		key, ok := pairs[i].(string)
		if !ok {
			continue
		}
		details[key] = pairs[i+1]
	}
	if len(details) == 0 {
		return nil
	}
	return details
}
