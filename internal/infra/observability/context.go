package observability

import "context"

type contextKey string

const (
	requestIDKey contextKey = "request_id"
	traceIDKey   contextKey = "trace_id"
	spanIDKey    contextKey = "span_id"
)

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

func RequestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(requestIDKey).(string)
	return v
}

func WithTrace(ctx context.Context, traceID, spanID string) context.Context {
	ctx = context.WithValue(ctx, traceIDKey, traceID)
	ctx = context.WithValue(ctx, spanIDKey, spanID)
	return ctx
}

func TraceFromContext(ctx context.Context) (traceID, spanID string) {
	traceID, _ = ctx.Value(traceIDKey).(string)
	spanID, _ = ctx.Value(spanIDKey).(string)
	return traceID, spanID
}
