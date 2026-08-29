package otel

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	apiotel "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	apitrace "go.opentelemetry.io/otel/trace"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	"github.com/eannchen/go-backend-architecture/internal/observability"
)

func TestSpanFinish_SetsStatusAndRecordsErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus codes.Code
		wantEvent  bool
	}{
		{name: "success", wantStatus: codes.Ok},
		{
			name:       "client error remains successful for service health",
			err:        fmt.Errorf("handle request: %w", apperr.New(apperr.CodeInvalidArgument, "invalid input")),
			wantStatus: codes.Ok,
			wantEvent:  true,
		},
		{
			name:       "server error",
			err:        apperr.New(apperr.CodeInternal, "database unavailable"),
			wantStatus: codes.Error,
			wantEvent:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := installSpanRecorder(t)
			_, span := NewTracer("test").Start(context.Background(), "test", tt.name)

			span.Finish(tt.err)

			ended := recorder.Ended()
			if len(ended) != 1 {
				t.Fatalf("ended spans = %d, want 1", len(ended))
			}
			if got := ended[0].Status().Code; got != tt.wantStatus {
				t.Fatalf("span status = %s, want %s", got, tt.wantStatus)
			}
			if got := hasExceptionEvent(ended[0]); got != tt.wantEvent {
				t.Fatalf("exception event = %v, want %v", got, tt.wantEvent)
			}
		})
	}
}

func TestTracerStartServer_RecordsKindAttributesAndIDs(t *testing.T) {
	recorder := installSpanRecorder(t)
	_, span := NewTracer("accounts-api").StartServer(
		context.Background(),
		"http",
		"GET /users",
		observability.FromPairs("http.request.method", "GET", "http.response.status_code", 200),
	)

	traceID, spanID, ok := span.IDs()
	if !ok || traceID == "" || spanID == "" {
		t.Fatalf("span IDs = %q, %q, %v; want valid IDs", traceID, spanID, ok)
	}
	span.Finish(nil)

	ended := recorder.Ended()
	if len(ended) != 1 {
		t.Fatalf("ended spans = %d, want 1", len(ended))
	}
	if ended[0].SpanKind() != apitrace.SpanKindServer {
		t.Fatalf("span kind = %s, want server", ended[0].SpanKind())
	}
	if got := traceAttribute(ended[0], "http.request.method"); got.AsString() != "GET" {
		t.Fatalf("request method attribute = %v, want GET", got)
	}
	if got := traceAttribute(ended[0], "http.response.status_code"); got.AsInt64() != 200 {
		t.Fatalf("status code attribute = %v, want 200", got)
	}
}

func TestTracerExtractHTTP_ContinuesRemoteTrace(t *testing.T) {
	headers := http.Header{}
	headers.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")

	ctx := NewTracer("accounts-api").ExtractHTTP(context.Background(), headers)
	spanContext := apitrace.SpanContextFromContext(ctx)

	if !spanContext.IsValid() || !spanContext.IsRemote() {
		t.Fatalf("extracted span context = %+v, want valid remote context", spanContext)
	}
	if got := spanContext.TraceID().String(); got != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("trace ID = %q, want propagated trace ID", got)
	}
}

func installSpanRecorder(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()

	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	previous := apiotel.GetTracerProvider()
	apiotel.SetTracerProvider(provider)
	t.Cleanup(func() {
		apiotel.SetTracerProvider(previous)
		_ = provider.Shutdown(context.Background())
	})

	return recorder
}

func hasExceptionEvent(span sdktrace.ReadOnlySpan) bool {
	for _, event := range span.Events() {
		if event.Name == "exception" {
			return true
		}
	}
	return false
}

func traceAttribute(span sdktrace.ReadOnlySpan, key string) attribute.Value {
	for _, item := range span.Attributes() {
		if string(item.Key) == key {
			return item.Value
		}
	}
	return attribute.Value{}
}
