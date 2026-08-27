package httpcontext

import (
	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
)

const (
	keyError         = "observability.error"
	keyErrorDetails  = "observability.error_details"
	keyTransportCode = "observability.transport_code"
	keyTransportMsg  = "observability.transport_message"
)

type Details = apperr.Details

func SetError(c *echo.Context, err error) {
	c.Set(keyError, err)
}

func SetErrorDetails(c *echo.Context, details Details) {
	c.Set(keyErrorDetails, details)
}

func Error(c *echo.Context) error {
	v, ok := c.Get(keyError).(error)
	if !ok || v == nil {
		return nil
	}
	return v
}

func ErrorDetails(c *echo.Context) Details {
	v, ok := c.Get(keyErrorDetails).(Details)
	if !ok || len(v) == 0 {
		return nil
	}
	return v
}

func SetTransportError(c *echo.Context, code string, message string) {
	c.Set(keyTransportCode, code)
	c.Set(keyTransportMsg, message)
}

func TransportError(c *echo.Context) (string, string) {
	code, _ := c.Get(keyTransportCode).(string)
	msg, _ := c.Get(keyTransportMsg).(string)
	return code, msg
}
