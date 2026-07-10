package main

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

	code := runLifecycle(ctx, cancel, lifecycle{
		start: func() error {
			return startErr
		},
		shutdown: func(context.Context) error {
			shutdownCalls++
			return nil
		},
		gracePeriod: time.Second,
		log:         logger.NoopLogger{},
	})

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if shutdownCalls != 1 {
		t.Fatalf("shutdown calls = %d, want 1", shutdownCalls)
	}
}
