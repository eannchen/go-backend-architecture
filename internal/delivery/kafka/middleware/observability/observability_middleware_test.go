package observability

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware"
	"github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware/recovery"
	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/logger/loggertest"
	appobservability "github.com/eannchen/go-backend-architecture/internal/observability"
)

// TestProcessingLogs protects completion severity, trace-context propagation, and
// panic diagnostics while excluding private record data from fields and stacks.
func TestProcessingLogs(t *testing.T) {
	for _, tc := range []struct {
		name       string
		err        error
		panicValue bool
		level      string
	}{
		{name: "success", level: "info"},
		{name: "invalid", err: middleware.ErrInvalidMessage, level: "warn"},
		{name: "error", err: errors.New("dependency failed"), level: "error"},
		{name: "timeout", err: context.DeadlineExceeded, level: "error"},
		{name: "canceled", err: context.Canceled, level: "info"},
		{name: "panic", panicValue: true, level: "info"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			log := &loggertest.Logger{
				InfoFunc:         func(context.Context, string, ...logger.Fields) {},
				WarnFunc:         func(context.Context, string, ...logger.Fields) {},
				ErrorFunc:        func(context.Context, string, error, ...logger.Fields) {},
				ErrorNoStackFunc: func(context.Context, string, error, ...logger.Fields) {},
			}
			type contextKey struct{}
			ctx := context.WithValue(t.Context(), contextKey{}, "processing")
			handler := middleware.Chain(func(context.Context, *kgo.Record) error {
				if tc.panicValue {
					panic("secret payload")
				}
				return tc.err
			}, New(appobservability.NoopTracer{}, log, appobservability.NoopMeter{}).Handler("notification.consume"), recovery.New(log).Handler())
			record := &kgo.Record{Topic: "notifications", Partition: 2, Offset: 7, Key: []byte("secret key"), Value: []byte("secret payload"), Headers: []kgo.RecordHeader{{Key: "token", Value: []byte("secret header")}}}
			err := handler(ctx, record)
			want := tc.err
			if tc.panicValue {
				want = middleware.ErrHandlerPanic
			}
			if !errors.Is(err, want) {
				t.Fatalf("error=%v want=%v", err, want)
			}
			var fields logger.Fields
			var loggedCtx context.Context
			switch tc.level {
			case "info":
				if len(log.InfoCalls) != 1 || len(log.WarnCalls) != 0 || len(log.ErrorNoStackCalls) != 0 {
					t.Fatal("wrong completion severity")
				}
				fields = log.InfoCalls[0].Fields[0]
				loggedCtx = log.InfoCalls[0].Context
			case "warn":
				if len(log.InfoCalls) != 0 || len(log.WarnCalls) != 1 || len(log.ErrorNoStackCalls) != 0 {
					t.Fatal("wrong completion severity")
				}
				fields = log.WarnCalls[0].Fields[0]
				loggedCtx = log.WarnCalls[0].Context
			case "error":
				if len(log.InfoCalls) != 0 || len(log.WarnCalls) != 0 || len(log.ErrorNoStackCalls) != 1 {
					t.Fatal("wrong completion severity")
				}
				fields = log.ErrorNoStackCalls[0].Fields[0]
				loggedCtx = log.ErrorNoStackCalls[0].Context
			}
			if fields["app.kafka.outcome"] != tc.name || fields["messaging.kafka.offset"] != int64(7) || fields["app.kafka.operation"] != "notification.consume" || loggedCtx.Value(contextKey{}) != "processing" {
				t.Fatalf("completion fields=%v", fields)
			}
			diagnostics := fmt.Sprint(fields)
			if tc.panicValue {
				if len(log.ErrorCalls) != 1 {
					t.Fatal("missing recovery log")
				}
				stack, _ := log.ErrorCalls[0].Fields[0]["panic.stack"].(string)
				if !strings.Contains(stack, "TestProcessingLogs") {
					t.Fatal("missing panic location")
				}
				diagnostics += fmt.Sprint(log.ErrorCalls[0])
			} else if len(log.ErrorCalls) != 0 {
				t.Fatal("unexpected recovery log")
			}
			if strings.Contains(diagnostics, "secret") {
				t.Fatal("private record data in log")
			}
		})
	}
}
