package loggertest

import (
	"context"
	"sync"
	"testing"

	"github.com/eannchen/go-backend-architecture/internal/logger"
)

func TestLoggerWarn_RecordsConfiguredCall(t *testing.T) {
	warned := false
	log := &Logger{
		WarnFunc: func(context.Context, string, ...logger.Fields) { warned = true },
	}
	fields := logger.FromPairs("user.id", int64(42))

	log.Warn(context.Background(), "cache unavailable", fields)

	if !warned {
		t.Fatal("expected configured WarnFunc to be called")
	}
	if len(log.WarnCalls) != 1 {
		t.Fatalf("warn calls = %d, want 1", len(log.WarnCalls))
	}
	call := log.WarnCalls[0]
	if call.Message != "cache unavailable" || len(call.Fields) != 1 || call.Fields[0]["user.id"] != int64(42) {
		t.Fatalf("unexpected warn call: %+v", call)
	}
}

func TestLoggerWarn_PanicsWhenUnconfigured(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected an unconfigured Warn call to panic")
		}
	}()

	new(Logger).Warn(context.Background(), "unexpected")
}

func TestLoggerWarn_RecordsConcurrentCalls(t *testing.T) {
	log := &Logger{
		WarnFunc: func(context.Context, string, ...logger.Fields) {},
	}
	const workers = 32
	var wg sync.WaitGroup
	wg.Add(workers)

	for range workers {
		go func() {
			defer wg.Done()
			log.Warn(context.Background(), "concurrent warning")
		}()
	}
	wg.Wait()

	if len(log.WarnCalls) != workers {
		t.Fatalf("warn calls = %d, want %d", len(log.WarnCalls), workers)
	}
}
