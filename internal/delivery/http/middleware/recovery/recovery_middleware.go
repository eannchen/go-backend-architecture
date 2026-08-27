package recovery

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	"github.com/eannchen/go-backend-architecture/internal/delivery/http/httpcontext"
	httpresponse "github.com/eannchen/go-backend-architecture/internal/delivery/http/response"
	"github.com/eannchen/go-backend-architecture/internal/logger"
)

// Middleware converts panics into safe HTTP failures when a response can still be written.
type Middleware struct {
	log       logger.Logger
	responder httpresponse.Responder
}

func New(log logger.Logger, responder httpresponse.Responder) *Middleware {
	if log == nil {
		log = logger.NoopLogger{}
	}
	if responder == nil {
		responder = httpresponse.NewResponder()
	}
	return &Middleware{log: log, responder: responder}
}

func (m *Middleware) Handler() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) (err error) {
			defer func() {
				recovered := recover()
				if recovered == nil {
					return
				}
				if recovered == http.ErrAbortHandler {
					panic(recovered)
				}

				cause := panicError(recovered)
				req := c.Request()
				route := c.Path()
				if route == "" {
					route = req.URL.Path
				}
				m.log.Error(req.Context(), "HTTP panic recovered", cause, logger.FromPairs(
					"http.request.method", req.Method,
					"http.route", route,
					"panic.stack", string(debug.Stack()),
				))

				if responseCommitted(c) {
					httpcontext.SetError(c, cause)
					httpcontext.SetTransportError(c, string(apperr.CodeInternal), "internal server error")
					err = cause
					return
				}

				err = m.responder.Error(
					c,
					cause,
					httpresponse.Code(apperr.CodeInternal),
					"internal server error",
				)
			}()

			return next(c)
		}
	}
}

func responseCommitted(c *echo.Context) bool {
	response, err := echo.UnwrapResponse(c.Response())
	return err == nil && response.Committed
}

func panicError(recovered any) error {
	if err, ok := recovered.(error); ok {
		return fmt.Errorf("panic: %w", err)
	}
	return fmt.Errorf("panic: %v", recovered)
}
