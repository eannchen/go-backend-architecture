package health

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	openapi "github.com/eannchen/go-backend-architecture/internal/delivery/http/openapi/gen"
	httpresponse "github.com/eannchen/go-backend-architecture/internal/delivery/http/response"
	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/observability"
	usecasehealth "github.com/eannchen/go-backend-architecture/internal/usecase/health"
)

type request struct {
	Check string `query:"check" validate:"omitempty,health_check_mode"`
}

func NewHandler(log logger.Logger, tracer observability.Tracer, responder httpresponse.Responder, usecase usecasehealth.Usecase) *Handler {
	if tracer == nil {
		tracer = observability.NoopTracer{}
	}
	if responder == nil {
		responder = httpresponse.NewResponder(nil)
	}
	return &Handler{
		logger:    log,
		tracer:    tracer,
		responder: responder,
		usecase:   usecase,
	}
}

type Handler struct {
	logger    logger.Logger
	tracer    observability.Tracer
	responder httpresponse.Responder
	usecase   usecasehealth.Usecase
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
		return h.responder.InvalidQuery(c, err, "invalid query parameters")
	}
	if err := c.Validate(&req); err != nil {
		spanErr = err
		return h.responder.InvalidQuery(c, err, "invalid query parameters")
	}

	mode, _ := usecasehealth.ParseCheckMode(req.Check)

	result, err := h.usecase.Check(ctx, mode)
	if err != nil {
		spanErr = err
		if appErr, ok := apperr.As(err); ok && appErr.Code == apperr.CodeUnavailable {
			return h.responder.AppErrorWithPayload(c, err, toResponse(result))
		}
		return h.responder.AppError(c, err)
	}

	return h.responder.Success(c, http.StatusOK, toResponse(result))
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
		Vectorstore: openapi.HealthDependency{
			Status: result.Vector.Status,
		},
	}
}
