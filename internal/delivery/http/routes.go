package http

import "github.com/labstack/echo/v5"

// RouteRegistrar lets each handler/module register its own routes.
type RouteRegistrar interface {
	RegisterRoutes(e *echo.Echo)
}
