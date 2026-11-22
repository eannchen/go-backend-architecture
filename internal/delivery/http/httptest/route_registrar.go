package httptest

import "github.com/labstack/echo/v5"

// RouteRegistrar is a configurable double for its parent package's RouteRegistrar contract.
type RouteRegistrar struct {
	RegisterRoutesFunc  func(e *echo.Echo)
	RegisterRoutesCalls []RouteRegistrarRegisterRoutesCall
}

// RouteRegistrarRegisterRoutesCall records one RouteRegistrar.RegisterRoutes invocation.
type RouteRegistrarRegisterRoutesCall struct {
	Echo *echo.Echo
}

func (d *RouteRegistrar) RegisterRoutes(e *echo.Echo) {
	d.RegisterRoutesCalls = append(d.RegisterRoutesCalls, RouteRegistrarRegisterRoutesCall{Echo: e})
	fn := d.RegisterRoutesFunc
	if fn == nil {
		panic("unexpected RouteRegistrar.RegisterRoutes call")
	}
	fn(e)
}
