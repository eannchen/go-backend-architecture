package http

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"vocynex-api/internal/apperr"
)

const (
	ErrCodeInvalidQuery = "INVALID_QUERY"
)

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func respondJSON(c *echo.Context, status int, payload any) error {
	return c.JSON(status, payload)
}

func respondError(c *echo.Context, status int, code string, message string, details any) error {
	return respondJSON(c, status, APIError{
		Code:    code,
		Message: message,
		Details: details,
	})
}

func respondInvalidQueryError(c *echo.Context, message string, details any) error {
	return respondError(c, http.StatusBadRequest, ErrCodeInvalidQuery, message, details)
}

func respondAppError(c *echo.Context, err error) error {
	appErr, ok := apperr.As(err)
	if !ok {
		return respondError(c, http.StatusInternalServerError, string(apperr.CodeInternal), "internal server error", nil)
	}
	return respondError(c, toHTTPStatus(err), string(appErr.Code), appErr.Message, appErr.Details)
}

func toHTTPStatus(err error) int {
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
