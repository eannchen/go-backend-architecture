package httptest

import "github.com/labstack/echo/v5"

// RouteRegistrar is the canonical configurable route-registration double.
type RouteRegistrar struct {
	RegisterRoutesFunc  func(*echo.Echo)
	RegisterRoutesCalls int
}

func (r *RouteRegistrar) RegisterRoutes(e *echo.Echo) {
	r.RegisterRoutesCalls++
	if r.RegisterRoutesFunc == nil {
		panic("unexpected RouteRegistrar.RegisterRoutes call")
	}
	r.RegisterRoutesFunc(e)
}
