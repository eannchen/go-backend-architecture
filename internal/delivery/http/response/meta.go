package response

import (
	"encoding/json"
	"fmt"

	"github.com/labstack/echo/v5"
)

const (
	keyError        = "observability.error"
	keyErrorDetails = "observability.error_details"
)

// Meta defines transport response metadata access for HTTP components.
type Meta interface {
	GetError(c *echo.Context) error
	GetErrorDetails(c *echo.Context) Details
	SetError(c *echo.Context, err error)
	SetErrorDetails(c *echo.Context, details Details)
}

type Details map[string]any

func (d Details) String() string {
	if len(d) == 0 {
		return ""
	}
	b, err := json.Marshal(d)
	if err != nil {
		return fmt.Sprintf("%v", map[string]any(d))
	}
	return string(b)
}

// ContextMeta stores and reads response metadata from Echo context.
type ContextMeta struct{}

// NewContextMeta creates a Meta backed by Echo request context.
func NewContextMeta() Meta {
	return ContextMeta{}
}

func (ContextMeta) SetError(c *echo.Context, err error) {
	c.Set(keyError, err)
}

func (ContextMeta) SetErrorDetails(c *echo.Context, details Details) {
	c.Set(keyErrorDetails, details)
}

func (ContextMeta) GetError(c *echo.Context) error {
	v, ok := c.Get(keyError).(error)
	if !ok || v == nil {
		return nil
	}
	return v
}

func (ContextMeta) GetErrorDetails(c *echo.Context) Details {
	v, ok := c.Get(keyErrorDetails).(Details)
	if !ok || len(v) == 0 {
		return nil
	}
	return v
}
