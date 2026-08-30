package main

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/logger/loggertest"
)

func TestRunLifecycle(t *testing.T) {
	startErr := errors.New("listen failed")
	shutdownErr := errors.New("shutdown failed")
	tests := []struct {
		name            string
		startErr        error
		shutdownErr     error
		wantCode        int
		wantLogMessages []string
	}{
		{
			name:     "server closed during graceful shutdown",
			startErr: http.ErrServerClosed,
			wantCode: 0,
		},
		{
			name:            "fatal server start error",
			startErr:        startErr,
			wantCode:        1,
			wantLogMessages: []string{"server exited with error"},
		},
		{
			name:            "graceful shutdown error",
			startErr:        http.ErrServerClosed,
			shutdownErr:     shutdownErr,
			wantCode:        1,
			wantLogMessages: []string{"graceful shutdown failed"},
		},
		{
			name:            "start and shutdown errors",
			startErr:        startErr,
			shutdownErr:     shutdownErr,
			wantCode:        1,
			wantLogMessages: []string{"server exited with error", "graceful shutdown failed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shutdownCalls := 0
			ctx, cancel := context.WithCancel(context.Background())
			log := &loggertest.Logger{
				ErrorFunc: func(context.Context, string, error, ...logger.Fields) {},
			}

			code := runLifecycle(ctx, cancel, lifecycle{
				start: func() error {
					return tt.startErr
				},
				shutdown: func(context.Context) error {
					shutdownCalls++
					return tt.shutdownErr
				},
				gracePeriod: time.Second,
				log:         log,
			})

			if code != tt.wantCode {
				t.Fatalf("exit code = %d, want %d", code, tt.wantCode)
			}
			if shutdownCalls != 1 {
				t.Fatalf("shutdown calls = %d, want 1", shutdownCalls)
			}
			if len(log.ErrorCalls) != len(tt.wantLogMessages) {
				t.Fatalf("error log calls = %d, want %d", len(log.ErrorCalls), len(tt.wantLogMessages))
			}
			for i, wantMessage := range tt.wantLogMessages {
				if got := log.ErrorCalls[i].Message; got != wantMessage {
					t.Fatalf("error log call %d message = %q, want %q", i, got, wantMessage)
				}
			}
		})
	}
}
