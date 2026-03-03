package http

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"vocynex-api/internal/usecase"
)

type HealthHandler struct {
	healthChecker usecase.HealthChecker
}

func NewHealthHandler(healthChecker usecase.HealthChecker) *HealthHandler {
	return &HealthHandler{healthChecker: healthChecker}
}

func (h *HealthHandler) GetHealth(c *echo.Context) error {
	status, err := h.healthChecker.Check(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusServiceUnavailable, status)
	}
	return c.JSON(http.StatusOK, status)
}
