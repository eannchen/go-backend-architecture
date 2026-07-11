package ratelimitmw

import (
	"strconv"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	httpresponse "github.com/eannchen/go-backend-architecture/internal/delivery/http/response"
	"github.com/eannchen/go-backend-architecture/internal/usecase/globalratelimit"
	"github.com/labstack/echo/v5"
)

type GlobalRateLimitMiddleware struct {
	limiter   globalratelimit.Limiter
	responder httpresponse.Responder
}

func NewGlobalRateLimit(limiter globalratelimit.Limiter, responder httpresponse.Responder) *GlobalRateLimitMiddleware {
	if responder == nil {
		responder = httpresponse.NewResponder(nil)
	}
	return &GlobalRateLimitMiddleware{limiter, responder}
}

func (m *GlobalRateLimitMiddleware) Handler() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			d, err := m.limiter.AllowIP(c.Request().Context(), c.RealIP())
			if err != nil {
				return m.responder.AppError(c, err)
			}
			if !d.Allowed {
				if d.RetryAfter > 0 {
					c.Response().Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds(d.RetryAfter)))
				}
				return m.responder.AppError(c, apperr.New(apperr.CodeTooManyRequests, "too many requests; please slow down"))
			}
			return next(c)
		}
	}
}

func retryAfterSeconds(d time.Duration) int {
	n := int(d.Round(time.Second).Seconds())
	if n < 1 {
		return 1
	}
	return n
}
