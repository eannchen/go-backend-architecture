package health

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"go-backend-architecture/internal/apperr"
	deliveryhttp "go-backend-architecture/internal/delivery/http"
	openapi "go-backend-architecture/internal/delivery/http/openapi/gen"
	"go-backend-architecture/internal/logger"
	"go-backend-architecture/internal/observability"
	usecasehealth "go-backend-architecture/internal/usecase/health"
)

type request struct {
	Check string `query:"check" validate:"omitempty,health_check_mode"`
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

func toResponse(result usecasehealth.Result) openapi.HealthResponse {
	return openapi.HealthResponse{
		Database: openapi.HealthDatabase{
			Status:        result.Database.Status,
			Name:          result.Database.Name,
			InRecovery:    result.Database.InRecovery,
			UptimeSeconds: result.Database.UptimeSeconds,
		},
		Cache: openapi.HealthDependency{
			Status: result.Cache.Status,
		},
		Kvstore: openapi.HealthDependency{
			Status: result.KVStore.Status,
		},
	}
}
