package sessionmw

import (
	"github.com/labstack/echo/v5"

	authhttp "github.com/eannchen/go-backend-architecture/internal/delivery/http/handler/auth"
	httpresponse "github.com/eannchen/go-backend-architecture/internal/delivery/http/response"
	authsession "github.com/eannchen/go-backend-architecture/internal/usecase/auth/session"
)

// SessionMiddleware validates session cookies and injects session info into the request context.
type SessionMiddleware struct {
	session    authsession.SessionManager
	cookieName string
	responder  httpresponse.Responder
}

// New creates a session middleware.
func New(session authsession.SessionManager, cookieName string, responder httpresponse.Responder) *SessionMiddleware {
	if responder == nil {
		responder = httpresponse.NewResponder(nil)
	}
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

			authhttp.SetSessionContext(c, authhttp.SessionInfo{
				UserID: sess.UserID,
				Email:  sess.Email,
				Method: sess.Method,
			})

			return next(c)
		}
	}
}
