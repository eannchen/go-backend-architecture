package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/eannchen/go-backend-architecture/internal/logger/loggertest"
	"github.com/eannchen/go-backend-architecture/internal/observability/observabilitytest"
)

func TestRuntimeShutdownAttemptsEveryResourceAndJoinsErrors(t *testing.T) {
	observabilityErr := errors.New("observability shutdown failed")
	loggerErr := errors.New("logger sync failed")
	obs := &observabilitytest.Runtime{
		ShutdownFunc: func(context.Context) error {
			return observabilityErr
		},
	}
	log := &loggertest.Logger{
		SyncFunc: func() error {
			return loggerErr
		},
	}
	runtime := &Runtime{Observability: obs, Logger: log}

	err := runtime.Shutdown(context.Background())

	if !errors.Is(err, observabilityErr) || !errors.Is(err, loggerErr) {
		t.Fatalf("Shutdown() error = %v, want both resource errors", err)
	}
	if obs.ShutdownCalls != 1 {
		t.Fatalf("observability Shutdown() calls = %d, want 1", obs.ShutdownCalls)
	}
	if log.SyncCalls != 1 {
		t.Fatalf("logger Sync() calls = %d, want 1", log.SyncCalls)
	}
}
