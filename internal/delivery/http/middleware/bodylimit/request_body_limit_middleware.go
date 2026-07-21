// Package bodylimitmw provides request-body bounds for Echo handlers.
package bodylimitmw

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

// Middleware rejects declared oversized bodies and bounds streamed bodies.
type Middleware struct {
	maxBytes int64
}

// New creates request-body limit middleware for maxBytes.
func New(maxBytes int64) *Middleware {
	return &Middleware{maxBytes: maxBytes}
}

// Handler returns the Echo middleware that applies the request-body limit.
func (m *Middleware) Handler() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			req := c.Request()
			if req.ContentLength > m.maxBytes {
				return echo.NewHTTPError(http.StatusRequestEntityTooLarge, "request body too large")
			}
			if req.Body != nil {
				req.Body = http.MaxBytesReader(c.Response(), req.Body, m.maxBytes)
			}
			return next(c)
		}
	}
}
