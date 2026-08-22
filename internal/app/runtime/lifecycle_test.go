package runtime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/logger"
)

func TestRunLifecycleShutsDownAfterFatalStartError(t *testing.T) {
	startErr := errors.New("listen failed")
	shutdownCalls := 0
	ctx, cancel := context.WithCancel(context.Background())

	code := RunLifecycle(ctx, cancel, Lifecycle{
		Start: func() error {
			return startErr
		},
		Shutdown: func(context.Context) error {
			shutdownCalls++
			return nil
		},
		GracePeriod: time.Second,
		Logger:      logger.NoopLogger{},
		Component:   "test_server",
	})

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if shutdownCalls != 1 {
		t.Fatalf("shutdown calls = %d, want 1", shutdownCalls)
	}
}
