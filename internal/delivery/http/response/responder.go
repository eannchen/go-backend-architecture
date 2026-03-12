package response

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
)

// Responder writes transport responses and records metadata for observability middleware.
type Responder interface {
	Success(c *echo.Context, status int, payload any) error
	Error(c *echo.Context, err error, status int, code string, message string, details ...Details) error
	InvalidQuery(c *echo.Context, err error, message string, details ...Details) error
	AppError(c *echo.Context, err error) error
}

type responder struct {
	meta Meta
}

// NewResponder creates an injectable HTTP responder.
func NewResponder(meta Meta) Responder {
	if meta == nil {
		meta = NewContextMeta()
	}
	return &responder{meta: meta}
}

type errorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (r *responder) Success(c *echo.Context, status int, payload any) error {
	return c.JSON(status, payload)
}

func (r *responder) Error(c *echo.Context, err error, status int, code string, message string, details ...Details) error {
	r.meta.SetError(c, err)
	r.meta.SetErrorDetails(c, optionalDetails(details...))
	return r.writeError(c, status, code, message)
}

func (r *responder) InvalidQuery(c *echo.Context, err error, message string, details ...Details) error {
	r.meta.SetError(c, err)
	r.meta.SetErrorDetails(c, optionalDetails(details...))
	return r.writeError(c, http.StatusBadRequest, "INVALID_QUERY", message)
}

func (r *responder) AppError(c *echo.Context, err error) error {
	r.meta.SetError(c, err)
	appErr, ok := apperr.As(err)
	if !ok {
		return r.writeError(c, http.StatusInternalServerError, string(apperr.CodeInternal), "internal server error")
	}
	r.meta.SetErrorDetails(c, Details(appErr.Details))
	return r.writeError(c, appErrCodeToStatusCode(appErr.Code), string(appErr.Code), appErr.Message)
}

func (r *responder) writeError(c *echo.Context, status int, code string, message string) error {
	return c.JSON(status, errorPayload{
		Code:    code,
		Message: message,
	})
}

func appErrCodeToStatusCode(code apperr.Code) int {
	switch code {
	case apperr.CodeInvalidArgument:
		return http.StatusBadRequest
	case apperr.CodeUnauthorized:
		return http.StatusUnauthorized
	case apperr.CodeForbidden:
		return http.StatusForbidden
	case apperr.CodeNotFound:
		return http.StatusNotFound
	case apperr.CodeConflict:
		return http.StatusConflict
	case apperr.CodeTooManyRequests:
		return http.StatusTooManyRequests
	case apperr.CodeUnavailable:
		return http.StatusServiceUnavailable
	case apperr.CodeTimeout:
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
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
