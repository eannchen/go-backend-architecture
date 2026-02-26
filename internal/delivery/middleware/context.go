package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/labstack/echo/v4"

	"vocynex-api/internal/infra/observability"
)

const requestIDHeader = "X-Request-ID"

func ContextPropagation(timeout time.Duration) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			reqCtx := req.Context()

			requestID := req.Header.Get(requestIDHeader)
			if requestID == "" {
				requestID = randomID()
			}

			reqCtx = observability.WithRequestID(reqCtx, requestID)
			c.Response().Header().Set(requestIDHeader, requestID)

			if timeout > 0 {
				var cancel context.CancelFunc
				reqCtx, cancel = context.WithTimeout(reqCtx, timeout)
				defer cancel()
			}

			c.SetRequest(req.WithContext(reqCtx))
			return next(c)
		}
	}
}

func randomID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
