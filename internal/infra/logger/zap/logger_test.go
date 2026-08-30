package zaplogger

import (
	"context"
	"errors"
	"strings"
	"testing"

	uzap "go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	"github.com/eannchen/go-backend-architecture/internal/logger"
)

type sinkCall struct {
	severity logger.Severity
	message  string
	fields   logger.Fields
}

func TestNew_RejectsInvalidLevels(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.LogConfig
	}{
		{name: "primary level", cfg: config.LogConfig{Level: "invalid", OTELevel: "info"}},
		{name: "sink level", cfg: config.LogConfig{Level: "info", OTELevel: "invalid"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New(tt.cfg); err == nil {
				t.Fatal("expected invalid log level to return an error")
			}
		})
	}
}

func TestLoggerSink_FiltersAndMergesFields(t *testing.T) {
	log, _ := newObservedLogger(zapcore.WarnLevel)
	log.SetContextFieldsProvider(func(context.Context) logger.Fields {
		return logger.FromPairs("trace.id", "trace-1", "source", "context")
	})
	var calls []sinkCall
	log.SetLogSink(func(_ context.Context, severity logger.Severity, message string, fields ...logger.Fields) {
		calls = append(calls, sinkCall{severity: severity, message: message, fields: logger.OptionalFields(fields...)})
	})

	log.Info(context.Background(), "filtered")
	log.Warn(context.Background(), "cache unavailable", logger.FromPairs("source", "call", "user.id", int64(42)))

	if len(calls) != 1 {
		t.Fatalf("sink calls = %d, want 1", len(calls))
	}
	call := calls[0]
	if call.severity != logger.SeverityWarn || call.message != "cache unavailable" {
		t.Fatalf("unexpected sink call: %+v", call)
	}
	if call.fields["trace.id"] != "trace-1" || call.fields["source"] != "call" || call.fields["user.id"] != int64(42) {
		t.Fatalf("unexpected merged fields: %+v", call.fields)
	}
	if call.fields["code.location"] == "" || call.fields["code.function"] == "" {
		t.Fatalf("expected caller fields, got %+v", call.fields)
	}
}

func TestLoggerError_SendsErrorAndApplicationCaller(t *testing.T) {
	log, observed := newObservedLogger(zapcore.DebugLevel)
	wantErr := errors.New("database unavailable")
	var call sinkCall
	log.SetLogSink(func(_ context.Context, severity logger.Severity, message string, fields ...logger.Fields) {
		call = sinkCall{severity: severity, message: message, fields: logger.OptionalFields(fields...)}
	})

	logErrorFromHelper(log, wantErr)

	if call.severity != logger.SeverityError || call.fields["error"] != wantErr {
		t.Fatalf("unexpected error sink call: %+v", call)
	}
	if got := call.fields["code.function"].(string); !strings.HasSuffix(got, ".logErrorFromHelper") {
		t.Fatalf("sink caller = %q, want logErrorFromHelper", got)
	}
	entries := observed.All()
	if len(entries) != 1 {
		t.Fatalf("primary log entries = %d, want 1", len(entries))
	}
	if got := entries[0].Caller.Function; !strings.HasSuffix(got, ".logErrorFromHelper") {
		t.Fatalf("primary caller = %q, want logErrorFromHelper", got)
	}
}

func newObservedLogger(sinkLevel zapcore.Level) (*impl, *observer.ObservedLogs) {
	core, observed := observer.New(zapcore.DebugLevel)
	return newLogger(uzap.New(core), sinkLevel), observed
}

func logErrorFromHelper(log logger.Logger, err error) {
	log.Error(context.Background(), "request failed", err)
}
