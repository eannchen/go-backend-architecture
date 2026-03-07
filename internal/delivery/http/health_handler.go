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

type healthCheckRequest struct {
	Check string `query:"check" validate:"omitempty,health_check_mode"`
}

type healthResponse struct {
	Database healthResponseDatabase `json:"database"`
}

type healthResponseDatabase struct {
	Status        string `json:"status"`
	Name          string `json:"name"`
	InRecovery    bool   `json:"in_recovery"`
	UptimeSeconds int64  `json:"uptime_seconds"`
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

	var req healthCheckRequest
	if err := c.Bind(&req); err != nil {
		spanErr = err
		return respondInvalidQueryError(c, "invalid query parameters", err.Error())
	}
	if err := c.Validate(&req); err != nil {
		spanErr = err
		return respondInvalidQueryError(c, "invalid query parameters", err.Error())
	}

	mode, _ := usecase.ParseHealthCheckMode(req.Check)

	result, err := h.healthChecker.Check(ctx, mode)
	if err != nil {
		spanErr = err
		statusCode := toHTTPStatus(err)
		if statusCode == http.StatusServiceUnavailable {
			return respondJSON(c, statusCode, toHealthResponse(result))
		}
		return respondAppError(c, err)
	}

	return respondJSON(c, http.StatusOK, toHealthResponse(result))
}

func toHealthResponse(result usecase.HealthCheckResult) healthResponse {
	return healthResponse{
		Database: healthResponseDatabase{
			Status:        result.Database.Status,
			Name:          result.Database.Name,
			InRecovery:    result.Database.InRecovery,
			UptimeSeconds: result.Database.UptimeSeconds,
		},
	}
}
