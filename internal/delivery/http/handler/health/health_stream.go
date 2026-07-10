package health

import (
	"context"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/delivery/http/response"
	"github.com/eannchen/go-backend-architecture/internal/logger"
	usecasehealth "github.com/eannchen/go-backend-architecture/internal/usecase/health"
)

// StreamPath is the bounded Server-Sent Events health demonstration route.
const StreamPath = "/health/stream"

// StreamConfig controls the health stream cadence and maximum lifetime.
type StreamConfig struct {
	CheckInterval     time.Duration
	HeartbeatInterval time.Duration
	MaxDuration       time.Duration
}

// StreamHealth sends immediate and periodic health events until the stream ends.
func (h *Handler) StreamHealth(c *echo.Context) (err error) {
	ctx, span := h.tracer.Start(c.Request().Context(), "handler", "health_handler.stream_health")
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

	stream, err := h.responder.StartSSE(c)
	if err != nil {
		spanErr = err
		return h.responder.AppError(c, err)
	}

	streamCtx, cancel := context.WithTimeout(ctx, h.stream.MaxDuration)
	defer cancel()
	if !h.emitHealth(streamCtx, stream, mode) {
		return nil
	}

	checks := time.NewTicker(h.stream.CheckInterval)
	defer checks.Stop()
	heartbeats := time.NewTicker(h.stream.HeartbeatInterval)
	defer heartbeats.Stop()

	for {
		select {
		case <-streamCtx.Done():
			return nil
		case <-checks.C:
			if !h.emitHealth(streamCtx, stream, mode) {
				return nil
			}
		case <-heartbeats.C:
			if err := stream.Comment("keep-alive"); err != nil {
				h.logger.Warn(streamCtx, "write health stream keep-alive failed", logger.FromPairs("error", err))
				return nil
			}
		}
	}
}

func (h *Handler) emitHealth(ctx context.Context, stream *response.SSEStream, mode usecasehealth.CheckMode) bool {
	result, checkErr := h.usecase.Check(ctx, mode)
	if checkErr != nil {
		// A dependency outage is the payload this demo should expose. Keep the
		// connection open so clients can observe recovery without reconnecting.
		h.logger.Warn(ctx, "health stream check failed", logger.FromPairs("error", checkErr))
	}
	if err := stream.Event("health", toResponse(result)); err != nil {
		h.logger.Warn(ctx, "write health stream event failed", logger.FromPairs("error", err))
		return false
	}
	return true
}
