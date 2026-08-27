package observability

import (
	"context"
	"time"

	googlegrpc "google.golang.org/grpc"

	"github.com/eannchen/go-backend-architecture/internal/logger"
	appobservability "github.com/eannchen/go-backend-architecture/internal/observability"
)

// Interceptor coordinates tracing, metrics, and access logging around each RPC.
type Interceptor struct {
	tracing   *Tracing
	metrics   *RequestMetrics
	accessLog *AccessLog
}

// New creates gRPC observability interceptors.
func New(tracer appobservability.Tracer, log logger.Logger, meter appobservability.Meter) *Interceptor {
	return &Interceptor{
		tracing:   NewTracing(tracer),
		metrics:   NewRequestMetrics(meter),
		accessLog: NewAccessLog(log),
	}
}

// Unary builds the unary RPC observability lifecycle.
func (i *Interceptor) Unary() googlegrpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		serverInfo *googlegrpc.UnaryServerInfo,
		handler googlegrpc.UnaryHandler,
	) (any, error) {
		rpc := newRPCInfo(serverInfo.FullMethod, "unary")
		ctx, span := i.tracing.Start(ctx, rpc)
		started := time.Now()

		response, handlerErr := handler(ctx, req)
		outcome := newRPCOutcome(rpc, time.Since(started), handlerErr)
		i.metrics.Record(ctx, outcome)
		i.accessLog.Record(ctx, outcome)
		i.tracing.Finish(span, outcome)
		return response, handlerErr
	}
}

// Stream builds the streaming RPC observability lifecycle.
func (i *Interceptor) Stream() googlegrpc.StreamServerInterceptor {
	return func(
		srv any,
		stream googlegrpc.ServerStream,
		serverInfo *googlegrpc.StreamServerInfo,
		handler googlegrpc.StreamHandler,
	) error {
		rpc := newRPCInfo(serverInfo.FullMethod, streamType(serverInfo))
		ctx, span := i.tracing.Start(stream.Context(), rpc)
		started := time.Now()
		i.metrics.StreamStarted(ctx, rpc)

		handlerErr := handler(srv, &contextServerStream{ServerStream: stream, ctx: ctx})
		outcome := newRPCOutcome(rpc, time.Since(started), handlerErr)
		i.metrics.StreamFinished(ctx, outcome)
		i.accessLog.Record(ctx, outcome)
		i.tracing.Finish(span, outcome)
		return handlerErr
	}
}
