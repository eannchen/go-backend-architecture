package http

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"vocynex-api/internal/infra/logger"
	"vocynex-api/internal/infra/observability"
	"vocynex-api/internal/usecase"
)

type HealthHandler struct {
	healthChecker usecase.HealthChecker
	logger        logger.Logger
}

func NewHealthHandler(healthChecker usecase.HealthChecker, log logger.Logger) *HealthHandler {
	return &HealthHandler{
		healthChecker: healthChecker,
		logger:        log,
	}
}

func (h *HealthHandler) GetHealth(c *echo.Context) error {
	ctx, span := observability.StartSpan(c.Request().Context(), "vocynex-api/handler", "health_handler.get_health")
	defer span.End()

	status, err := h.healthChecker.Check(ctx)
	if err != nil {
		span.Fail(err, err.Error())
		h.logger.Warn(ctx, "health endpoint returned degraded status")
		return c.JSON(http.StatusServiceUnavailable, status)
	}
	span.OK()
	h.logger.Debug(ctx, "health endpoint returned ok")
	return c.JSON(http.StatusOK, status)
}
