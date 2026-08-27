package otel

import (
	"context"
	"fmt"
	"testing"

	apiotel "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
)

func TestSpanFinish_ClientErrorIsRecordedWithoutErrorStatus(t *testing.T) {
	recorder := installSpanRecorder(t)

	_, span := NewTracer("test").Start(context.Background(), "test", "client error")
	span.Finish(fmt.Errorf("handle request: %w", apperr.New(apperr.CodeInvalidArgument, "invalid input")))

	ended := recorder.Ended()
	if len(ended) != 1 {
		t.Fatalf("ended spans = %d, want 1", len(ended))
	}
	if got := ended[0].Status().Code; got != codes.Ok {
		t.Fatalf("span status = %s, want %s", got, codes.Ok)
	}
	if !hasExceptionEvent(ended[0]) {
		t.Fatal("expected client error to be recorded on the span")
	}
}

func TestSpanFinish_ServerErrorHasErrorStatus(t *testing.T) {
	recorder := installSpanRecorder(t)

	_, span := NewTracer("test").Start(context.Background(), "test", "server error")
	span.Finish(apperr.New(apperr.CodeInternal, "database unavailable"))

	ended := recorder.Ended()
	if len(ended) != 1 {
		t.Fatalf("ended spans = %d, want 1", len(ended))
	}
	if got := ended[0].Status().Code; got != codes.Error {
		t.Fatalf("span status = %s, want %s", got, codes.Error)
	}
}

func TestTracerExtractsRemoteParentFromTextCarrier(t *testing.T) {
	recorder := installSpanRecorder(t)
	carrier := propagation.MapCarrier{
		"traceparent": "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01",
	}

	ctx := NewTracer("test").Extract(context.Background(), carrier)
	_, span := NewTracer("test").StartServer(ctx, "grpc", "/test.Service/Check")
	span.Finish(nil)

	ended := recorder.Ended()
	if len(ended) != 1 {
		t.Fatalf("ended spans = %d, want 1", len(ended))
	}
	if got := ended[0].Parent().TraceID().String(); got != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("parent trace ID = %q", got)
	}
}

func installSpanRecorder(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()

	recorder := tracetest.NewSpanRecorder()
	provider := trace.NewTracerProvider(trace.WithSpanProcessor(recorder))
	previous := apiotel.GetTracerProvider()
	apiotel.SetTracerProvider(provider)
	t.Cleanup(func() {
		apiotel.SetTracerProvider(previous)
		_ = provider.Shutdown(context.Background())
	})

	return recorder
}

func hasExceptionEvent(span trace.ReadOnlySpan) bool {
	for _, event := range span.Events() {
		if event.Name == "exception" {
			return true
		}
	}
	return false
}
