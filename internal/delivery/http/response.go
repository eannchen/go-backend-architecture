package http

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
)

const (
	ErrCodeInvalidQuery = "INVALID_QUERY"
)

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func RespondJSON(c *echo.Context, status int, payload any) error {
	return c.JSON(status, payload)
}

func RespondError(c *echo.Context, status int, code string, message string, details any) error {
	return RespondJSON(c, status, APIError{
		Code:    code,
		Message: message,
		Details: details,
	})
}

func RespondInvalidQueryError(c *echo.Context, message string, details any) error {
	return RespondError(c, http.StatusBadRequest, ErrCodeInvalidQuery, message, details)
}

func RespondAppError(c *echo.Context, err error) error {
	appErr, ok := apperr.As(err)
	if !ok {
		return RespondError(c, http.StatusInternalServerError, string(apperr.CodeInternal), "internal server error", nil)
	}
	return RespondError(c, ToStatusCode(err), string(appErr.Code), appErr.Message, appErr.Details)
}

func ToStatusCode(err error) int {
	if err == nil {
		return http.StatusOK
	}

	appErr, ok := apperr.As(err)
	if !ok {
		return http.StatusInternalServerError
	}

	switch appErr.Code {
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
