package auth

import (
	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/usecase/auth"
)

const sessionContextKey = "auth.session"

// SessionInfo holds session data stored in the request context by the session middleware.
type SessionInfo struct {
	UserID int64
	Email  string
	Method auth.MethodType
}

// SetSessionContext stores session info in the Echo context for downstream handlers.
func SetSessionContext(c *echo.Context, info SessionInfo) {
	c.Set(sessionContextKey, info)
}

// SessionFromContext retrieves the session info stored by the session middleware.
func SessionFromContext(c *echo.Context) (SessionInfo, bool) {
	v := c.Get(sessionContextKey)
	if v == nil {
		return SessionInfo{}, false
	}
	info, ok := v.(SessionInfo)
	return info, ok
}
