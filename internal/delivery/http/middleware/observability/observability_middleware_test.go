package observabilitymw

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/delivery/http/httpcontext"
	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/logger/loggertest"
	"github.com/eannchen/go-backend-architecture/internal/observability"
	"github.com/eannchen/go-backend-architecture/internal/observability/observabilitytest"
)

func TestErrorCauseChain(t *testing.T) {
	root := errors.New("root")
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "nil", want: ""},
		{name: "single", err: root, want: "root"},
		{name: "wrapped", err: fmt.Errorf("wrapped: %w", root), want: "wrapped: root; root"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := errorCauseChain(tt.err); got != tt.want {
				t.Fatalf("cause chain = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAccessLogMiddlewareRecordsRequestOutcome(t *testing.T) {
	tests := []struct {
		name          string
		status        int
		originalErr   error
		handlerErr    error
		wantInfoCalls int
		wantErrCalls  int
	}{
		{name: "successful request", status: http.StatusNoContent, wantInfoCalls: 1},
		{name: "server error", status: http.StatusServiceUnavailable, originalErr: errors.New("database unavailable"), wantErrCalls: 1},
		{name: "raw handler error", status: http.StatusInternalServerError, handlerErr: errors.New("handler failed"), wantErrCalls: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := &loggertest.Logger{
				InfoFunc:         func(context.Context, string, ...logger.Fields) {},
				ErrorNoStackFunc: func(context.Context, string, error, ...logger.Fields) {},
			}
			meta := httpcontext.ContextMeta{}
			e := echo.New()
			c := e.NewContext(httptest.NewRequest(http.MethodGet, "/users/42", nil), httptest.NewRecorder())
			c.SetPath("/users/:id")
			handler := NewAccessLogMiddleware(log, meta).Handler()(func(c *echo.Context) error {
				if tt.originalErr != nil {
					meta.SetError(c, tt.originalErr)
					meta.SetErrorDetails(c, httpcontext.Details{"dependency": "postgres"})
					meta.SetTransportError(c, "UNAVAILABLE", "database unavailable")
				}
				if tt.handlerErr != nil {
					return tt.handlerErr
				}
				return c.NoContent(tt.status)
			})

			if err := handler(c); err != tt.handlerErr {
				t.Fatalf("handler error = %v, want %v", err, tt.handlerErr)
			}
			if len(log.InfoCalls) != tt.wantInfoCalls || len(log.ErrorNoStackCalls) != tt.wantErrCalls {
				t.Fatalf("log calls = info:%d error:%d, want info:%d error:%d", len(log.InfoCalls), len(log.ErrorNoStackCalls), tt.wantInfoCalls, tt.wantErrCalls)
			}

			var fields logger.Fields
			if len(log.InfoCalls) == 1 {
				fields = log.InfoCalls[0].Fields[0]
			} else {
				fields = log.ErrorNoStackCalls[0].Fields[0]
				wantErr := tt.originalErr
				if wantErr == nil {
					wantErr = tt.handlerErr
				}
				if log.ErrorNoStackCalls[0].Err != wantErr {
					t.Fatalf("logged error = %v, want %v", log.ErrorNoStackCalls[0].Err, wantErr)
				}
			}
			if fields[keyHTTPRoute] != "/users/:id" || fields[keyURLPath] != "/users/42" || fields[keyHTTPResponseStatus] != tt.status {
				t.Fatalf("unexpected request fields: %+v", fields)
			}
			wantErr := tt.originalErr
			if wantErr == nil {
				wantErr = tt.handlerErr
			}
			if wantErr != nil && fields[keyError] != wantErr.Error() {
				t.Fatalf("unexpected error fields: %+v", fields)
			}
			if tt.originalErr != nil && fields[keyTransportCode] != "UNAVAILABLE" {
				t.Fatalf("unexpected transport fields: %+v", fields)
			}
		})
	}
}

func TestTraceMiddlewareRecordsSpanAndPropagatesIDs(t *testing.T) {
	handlerErr := errors.New("handler failed")
	span := &observabilitytest.Span{
		SetAttributesFunc: func(...observability.Fields) {},
		FinishFunc:        func(error, ...string) {},
		IDsFunc:           func() (string, string, bool) { return "trace-1", "span-1", true },
	}
	tracer := &observabilitytest.Tracer{
		ExtractHTTPFunc: func(ctx context.Context, _ http.Header) context.Context { return ctx },
		StartServerFunc: func(ctx context.Context, _, _ string, _ ...observability.Fields) (context.Context, observability.Span) {
			return ctx, span
		},
	}
	meta := httpcontext.ContextMeta{}
	e := echo.New()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/users/42", nil), httptest.NewRecorder())
	c.SetPath("/users/:id")
	handler := NewTraceMiddleware(tracer, meta).Handler()(func(c *echo.Context) error {
		traceID, spanID := observability.TraceFromContext(c.Request().Context())
		if traceID != "trace-1" || spanID != "span-1" {
			t.Fatalf("trace context = %q %q, want propagated IDs", traceID, spanID)
		}
		meta.SetErrorDetails(c, httpcontext.Details{"dependency": "postgres"})
		meta.SetTransportError(c, "INTERNAL", "internal server error")
		return handlerErr
	})

	if err := handler(c); err != handlerErr {
		t.Fatalf("handler error = %v, want %v", err, handlerErr)
	}
	if tracer.ExtractHTTPCalls != 1 || tracer.StartServerCalls != 1 {
		t.Fatalf("tracer calls = extract:%d start:%d, want one each", tracer.ExtractHTTPCalls, tracer.StartServerCalls)
	}
	if tracer.StartServerScope != "http" || tracer.StartServerSpanName != "GET /users/:id" {
		t.Fatalf("started span = %q %q", tracer.StartServerScope, tracer.StartServerSpanName)
	}
	startFields := tracer.StartServerFields[0]
	if startFields[keyHTTPRoute] != "/users/:id" || startFields[keyURLPath] != "/users/42" {
		t.Fatalf("unexpected start fields: %+v", startFields)
	}
	attributes := mergeSpanAttributes(span.SetAttributesCalls)
	if attributes[keyHTTPResponseStatus] != http.StatusInternalServerError || attributes[keyError] != handlerErr.Error() || attributes[keyTransportCode] != "INTERNAL" {
		t.Fatalf("unexpected final attributes: %+v", attributes)
	}
	if len(span.FinishCalls) != 1 || span.FinishCalls[0].Err != handlerErr {
		t.Fatalf("finish calls = %+v, want original error", span.FinishCalls)
	}
}

func TestRequestMetricsMiddlewareRecordsBoundedRouteAndError(t *testing.T) {
	meter := observabilitytest.NewRecordingMeter()
	e := echo.New()
	e.GET("/protected", NewRequestMetricsMiddleware(meter).Handler()(func(c *echo.Context) error {
		return c.NoContent(http.StatusUnauthorized)
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	requests := meter.CounterSamples("http_server_requests_total")
	if len(requests) != 1 || requests[0].Fields["http.route"] != "/protected" || requests[0].Fields["http.response.status_code"] != "401" {
		t.Fatalf("request metric = %#v", requests)
	}
	if errorSamples := meter.CounterSamples("http_server_errors_total"); len(errorSamples) != 1 {
		t.Fatalf("error metric count = %d, want 1", len(errorSamples))
	}
	if samples := meter.HistogramSamples("http_server_request_duration_seconds"); len(samples) != 1 {
		t.Fatalf("latency metric count = %d, want 1", len(samples))
	}
}

func mergeSpanAttributes(calls []observabilitytest.SetAttributesCall) observability.Fields {
	var sets []observability.Fields
	for _, call := range calls {
		sets = append(sets, call.Fields...)
	}
	return observability.MergeFields(sets...)
}
