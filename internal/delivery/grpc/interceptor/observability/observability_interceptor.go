package observability

import (
	"context"
	"time"

	googlegrpc "google.golang.org/grpc"

	"github.com/eannchen/go-backend-architecture/internal/logger"
	appobservability "github.com/eannchen/go-backend-architecture/internal/observability"
)

// Interceptor composes tracing, metrics, and access logging around each RPC.
type Interceptor struct {
	tracing   *Tracing
	metrics   *RequestMetrics
	accessLog *AccessLog
}

func New(tracer appobservability.Tracer, log logger.Logger, meter appobservability.Meter) *Interceptor {
	return &Interceptor{
		tracing:   NewTracing(tracer),
		metrics:   NewRequestMetrics(meter),
		accessLog: NewAccessLog(log),
	}
}

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

		response, err := handler(ctx, req)
		outcome := newRPCOutcome(rpc, time.Since(started), err)
		i.metrics.Record(ctx, outcome)
		i.accessLog.Record(ctx, outcome)
		i.tracing.Finish(span, outcome)
		return response, err
	}
}

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

		err := handler(srv, &contextServerStream{ServerStream: stream, ctx: ctx})
		outcome := newRPCOutcome(rpc, time.Since(started), err)
		i.metrics.StreamFinished(ctx, outcome)
		i.accessLog.Record(ctx, outcome)
		i.tracing.Finish(span, outcome)
		return err
	}
}
