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
	tracer        observability.Tracer
}

func NewHealthHandler(log logger.Logger, tracer observability.Tracer, healthChecker usecase.HealthChecker) *HealthHandler {
	return &HealthHandler{
		healthChecker: healthChecker,
		logger:        log,
		tracer:        tracer,
	}
}

func (h *HealthHandler) RegisterRoutes(e *echo.Echo) {
	e.GET("/healthz", h.GetHealth)
}

func (h *HealthHandler) GetHealth(c *echo.Context) (err error) {
	ctx, span := h.tracer.Start(c.Request().Context(), "handler", "health_handler.get_health")
	var spanErr error
	defer func() {
		span.Finish(spanErr)
	}()

	status, err := h.healthChecker.Check(ctx)
	if err != nil {
		spanErr = err
		h.logger.Warn(ctx, "health endpoint returned degraded status")
		return c.JSON(http.StatusServiceUnavailable, status)
	}
	h.logger.Debug(ctx, "health endpoint returned ok")
	return c.JSON(http.StatusOK, status)
}
