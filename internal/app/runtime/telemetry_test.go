package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/eannchen/go-backend-architecture/pkg/logger/loggertest"
	"github.com/eannchen/go-backend-architecture/pkg/observability/otel"
)

// TestTelemetryShutdownPreservesLoggerFailure verifies resource cleanup retains
// the logger's error. Provider shutdown failures are covered by the OTel runtime.
func TestTelemetryShutdownPreservesLoggerFailure(t *testing.T) {
	loggerErr := errors.New("logger sync failed")
	log := &loggertest.Logger{SyncFunc: func() error { return loggerErr }}
	obs, err := otel.Setup(t.Context(), otel.Config{}, "test", "test")
	if err != nil {
		t.Fatal(err)
	}
	telemetry := &Telemetry{Logger: log, Observability: obs}
	if err := telemetry.Shutdown(context.Background()); !errors.Is(err, loggerErr) {
		t.Fatalf("Shutdown() error = %v, want logger failure", err)
	}
	if len(log.SyncCalls) != 1 {
		t.Fatalf("Sync calls=%d", len(log.SyncCalls))
	}
}
