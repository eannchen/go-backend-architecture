package health

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"vocynex-api/internal/apperr"
	deliveryhttp "vocynex-api/internal/delivery/http"
	"vocynex-api/internal/infra/logger"
	"vocynex-api/internal/infra/observability"
	usecasehealth "vocynex-api/internal/usecase/health"
)

type request struct {
	Check string `query:"check" validate:"omitempty,health_check_mode"`
}

type response struct {
	Database Database `json:"database"`
}

type Database struct {
	Status        string `json:"status"`
	Name          string `json:"name"`
	InRecovery    bool   `json:"in_recovery"`
	UptimeSeconds int64  `json:"uptime_seconds"`
}

func NewHandler(log logger.Logger, tracer observability.Tracer, usecase usecasehealth.Usecase) *Handler {
	return &Handler{
		logger:  log,
		tracer:  tracer,
		usecase: usecase,
	}
}

type Handler struct {
	logger  logger.Logger
	tracer  observability.Tracer
	usecase usecasehealth.Usecase
}

func (h *Handler) RegisterRoutes(e *echo.Echo) {
	e.GET("/health", h.GetHealth)
}

func (h *Handler) GetHealth(c *echo.Context) (err error) {
	ctx, span := h.tracer.Start(c.Request().Context(), "handler", "health_handler.get_health")
	var spanErr error
	defer func() {
		span.Finish(spanErr)
	}()

	var req request
	if err := c.Bind(&req); err != nil {
		spanErr = err
		return deliveryhttp.RespondInvalidQueryError(c, "invalid query parameters", err.Error())
	}
	if err := c.Validate(&req); err != nil {
		spanErr = err
		return deliveryhttp.RespondInvalidQueryError(c, "invalid query parameters", err.Error())
	}

	mode, _ := usecasehealth.ParseCheckMode(req.Check)

	result, err := h.usecase.Check(ctx, mode)
	if err != nil {
		spanErr = err
		if appErr, ok := apperr.As(err); ok && appErr.Code == apperr.CodeUnavailable {
			return deliveryhttp.RespondJSON(c, deliveryhttp.ToStatusCode(appErr), toResponse(result))
		}
		return deliveryhttp.RespondAppError(c, err)
	}

	return deliveryhttp.RespondJSON(c, http.StatusOK, toResponse(result))
}

func toResponse(result usecasehealth.Result) response {
	return response{
		Database: Database{
			Status:        result.Database.Status,
			Name:          result.Database.Name,
			InRecovery:    result.Database.InRecovery,
			UptimeSeconds: result.Database.UptimeSeconds,
		},
	}
}
