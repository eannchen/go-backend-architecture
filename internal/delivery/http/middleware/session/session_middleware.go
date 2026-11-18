package sessionmw

import (
	"context"

	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/delivery/http/httpcontext"
	httpresponse "github.com/eannchen/go-backend-architecture/internal/delivery/http/response"
	"github.com/eannchen/go-backend-architecture/internal/usecase/auth"
)

// SessionValidator is the application behavior required by this adapter.
type SessionValidator interface {
	Validate(context.Context, string) (auth.Session, error)
}

// SessionMiddleware validates session cookies and injects session info into the request context.
type SessionMiddleware struct {
	session    SessionValidator
	cookieName string
	responder  *httpresponse.Responder
}

// New creates a session middleware.
func New(session SessionValidator, cookieName string, responder *httpresponse.Responder) *SessionMiddleware {
	return &SessionMiddleware{
		session:    session,
		cookieName: cookieName,
		responder:  responder,
	}
}

// Handler returns an Echo middleware that enforces authentication on protected routes.
func (m *SessionMiddleware) Handler() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			cookie, err := c.Cookie(m.cookieName)
			if err != nil || cookie == nil || cookie.Value == "" {
				return m.responder.Error(c, err, httpresponse.Code("UNAUTHORIZED"), "authentication required")
			}

			sess, err := m.session.Validate(c.Request().Context(), cookie.Value)
			if err != nil {
				return m.responder.AppError(c, err)
			}

			httpcontext.SetSessionContext(c, httpcontext.SessionInfo{
				UserID: sess.UserID,
				Email:  sess.Email,
				Method: sess.Method,
			})

			return next(c)
		}
	}
}
