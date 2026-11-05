package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/eannchen/go-backend-architecture/internal/logger/loggertest"
)

// TestRuntimeShutdownPreservesLoggerFailure verifies resource cleanup retains
// the logger's error. Provider shutdown failures are covered by the OTel runtime.
func TestRuntimeShutdownPreservesLoggerFailure(t *testing.T) {
	loggerErr := errors.New("logger sync failed")
	log := &loggertest.Logger{SyncFunc: func() error { return loggerErr }}
	runtime := &Runtime{Telemetry: &Telemetry{Logger: log}}
	if err := runtime.Shutdown(context.Background()); !errors.Is(err, loggerErr) {
		t.Fatalf("Shutdown() error = %v, want logger failure", err)
	}
	if log.SyncCalls != 1 {
		t.Fatalf("Sync calls=%d", log.SyncCalls)
	}
}
