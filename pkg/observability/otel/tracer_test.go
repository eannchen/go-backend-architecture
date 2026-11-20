package otel

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	apitrace "go.opentelemetry.io/otel/trace"

	"github.com/eannchen/go-backend-architecture/pkg/observability"
)

// TestSpanFinish_SetsStatusAndRecordsErrors checks span completion records status and errors for diagnostics.
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
			err:        fmt.Errorf("handle request: %w", clientTestError{}),
			wantStatus: codes.Ok,
			wantEvent:  true,
		},
		{
			name:       "server error",
			err:        errors.New("database unavailable"),
			wantStatus: codes.Error,
			wantEvent:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, recorder := newSpanRecorder(t)
			_, span := NewTracer(provider, "test").Start(context.Background(), "test", tt.name)

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

// TestTracerStartRecordsInternalKind checks ordinary spans use internal span kind.
func TestTracerStartRecordsInternalKind(t *testing.T) {
	provider, recorder := newSpanRecorder(t)
	_, span := NewTracer(provider, "accounts-api").Start(context.Background(), "usecase", "account.get")
	span.Finish(nil)

	ended := recorder.Ended()
	if len(ended) != 1 || ended[0].SpanKind() != apitrace.SpanKindInternal {
		t.Fatalf("ended spans = %#v, want one internal span", ended)
	}
}

// TestTracerStartServer_RecordsKindAttributesAndExposesContext checks server spans carry attributes and expose the started context.
func TestTracerStartServer_RecordsKindAttributesAndExposesContext(t *testing.T) {
	provider, recorder := newSpanRecorder(t)
	tracer := NewTracer(provider, "accounts-api")
	ctx, span := tracer.StartServer(
		context.Background(),
		"http",
		"GET /users",
		observability.FromPairs("http.request.method", "GET", "http.response.status_code", 200),
	)

	traceContext, ok := tracer.TraceContext(ctx)
	if !ok || traceContext.TraceID == "" || traceContext.SpanID == "" {
		t.Fatalf("trace context = %+v, %v; want valid IDs", traceContext, ok)
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

// TestTracerStartClientRecordsClientKind checks outbound calls create client-kind spans.
func TestTracerStartClientRecordsClientKind(t *testing.T) {
	provider, recorder := newSpanRecorder(t)
	_, span := NewTracer(provider, "accounts-api").StartClient(context.Background(), "grpc-client", "/test.Service/Check")
	span.Finish(nil)

	ended := recorder.Ended()
	if len(ended) != 1 || ended[0].SpanKind() != apitrace.SpanKindClient {
		t.Fatalf("ended spans = %#v, want one client span", ended)
	}
}

// TestTracerExtractContinuesRemoteTrace checks extraction continues a remote parent trace.
func TestTracerExtractContinuesRemoteTrace(t *testing.T) {
	provider, _ := newSpanRecorder(t)
	carrier := propagation.MapCarrier{
		"traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
	}

	ctx := NewTracer(provider, "accounts-api").Extract(context.Background(), carrier)
	spanContext := apitrace.SpanContextFromContext(ctx)

	if !spanContext.IsValid() || !spanContext.IsRemote() {
		t.Fatalf("extracted span context = %+v, want valid remote context", spanContext)
	}
	if got := spanContext.TraceID().String(); got != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("trace ID = %q, want propagated trace ID", got)
	}
}

// TestTracerExtractsRemoteParentFromTextCarrier checks a text carrier can supply the remote trace parent.
func TestTracerExtractsRemoteParentFromTextCarrier(t *testing.T) {
	provider, recorder := newSpanRecorder(t)
	carrier := propagation.MapCarrier{
		"traceparent": "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01",
	}

	ctx := NewTracer(provider, "test").Extract(context.Background(), carrier)
	_, span := NewTracer(provider, "test").StartServer(ctx, "grpc", "/test.Service/Check")
	span.Finish(nil)

	ended := recorder.Ended()
	if len(ended) != 1 {
		t.Fatalf("ended spans = %d, want 1", len(ended))
	}
	if got := ended[0].Parent().TraceID().String(); got != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("parent trace ID = %q", got)
	}
}

// TestTracerInjectsCurrentSpanIntoTextCarrier checks injection writes the current span context to a text carrier.
func TestTracerInjectsCurrentSpanIntoTextCarrier(t *testing.T) {
	provider, recorder := newSpanRecorder(t)
	ctx, span := NewTracer(provider, "test").StartClient(context.Background(), "grpc-client", "/test.Service/Check")
	carrier := propagation.MapCarrier{}

	NewTracer(provider, "test").Inject(ctx, carrier)
	span.Finish(nil)

	if carrier.Get("traceparent") == "" {
		t.Fatalf("carrier = %#v, want traceparent", carrier)
	}
	if len(recorder.Ended()) != 1 {
		t.Fatalf("ended spans = %d, want 1", len(recorder.Ended()))
	}
}

func newSpanRecorder(t *testing.T) (*sdktrace.TracerProvider, *tracetest.SpanRecorder) {
	t.Helper()

	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := provider.Shutdown(ctx); err != nil {
			t.Error(err)
		}
	})

	return provider, recorder
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

// clientTestError models the optional error classification recognized by Finish
// without coupling the library tests to an application's error package.
type clientTestError struct{}

func (clientTestError) Error() string       { return "invalid input" }
func (clientTestError) IsClientError() bool { return true }
