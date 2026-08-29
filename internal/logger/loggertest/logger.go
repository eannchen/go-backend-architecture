package loggertest

import (
	"context"
	"sync"

	"github.com/eannchen/go-backend-architecture/internal/logger"
)

// LogCall records one non-error logging invocation.
type LogCall struct {
	Context context.Context
	Message string
	Fields  []logger.Fields
}

// ErrorCall records one error logging invocation.
type ErrorCall struct {
	Context context.Context
	Message string
	Err     error
	Fields  []logger.Fields
}

// Logger is the canonical configurable double for logger.Logger.
type Logger struct {
	mu sync.Mutex

	DebugFunc  func(context.Context, string, ...logger.Fields)
	DebugCalls []LogCall
	InfoFunc   func(context.Context, string, ...logger.Fields)
	InfoCalls  []LogCall
	WarnFunc   func(context.Context, string, ...logger.Fields)
	WarnCalls  []LogCall

	ErrorFunc         func(context.Context, string, error, ...logger.Fields)
	ErrorCalls        []ErrorCall
	ErrorNoStackFunc  func(context.Context, string, error, ...logger.Fields)
	ErrorNoStackCalls []ErrorCall

	SetLogSinkFunc  func(logger.LogSinkFunc)
	SetLogSinkCalls int
	SetLogSinkSink  logger.LogSinkFunc

	SetContextFieldsProviderFunc     func(logger.ContextFieldsProviderFunc)
	SetContextFieldsProviderCalls    int
	SetContextFieldsProviderProvider logger.ContextFieldsProviderFunc

	SyncFunc  func() error
	SyncCalls int
}

func (l *Logger) Debug(ctx context.Context, message string, fields ...logger.Fields) {
	fn := l.recordLogCall(&l.DebugCalls, l.DebugFunc, ctx, message, fields)
	if fn == nil {
		panic("unexpected Logger.Debug call")
	}
	fn(ctx, message, fields...)
}

func (l *Logger) Info(ctx context.Context, message string, fields ...logger.Fields) {
	fn := l.recordLogCall(&l.InfoCalls, l.InfoFunc, ctx, message, fields)
	if fn == nil {
		panic("unexpected Logger.Info call")
	}
	fn(ctx, message, fields...)
}

func (l *Logger) Warn(ctx context.Context, message string, fields ...logger.Fields) {
	fn := l.recordLogCall(&l.WarnCalls, l.WarnFunc, ctx, message, fields)
	if fn == nil {
		panic("unexpected Logger.Warn call")
	}
	fn(ctx, message, fields...)
}

func (l *Logger) Error(ctx context.Context, message string, err error, fields ...logger.Fields) {
	fn := l.recordErrorCall(&l.ErrorCalls, l.ErrorFunc, ctx, message, err, fields)
	if fn == nil {
		panic("unexpected Logger.Error call")
	}
	fn(ctx, message, err, fields...)
}

func (l *Logger) ErrorNoStack(ctx context.Context, message string, err error, fields ...logger.Fields) {
	fn := l.recordErrorCall(&l.ErrorNoStackCalls, l.ErrorNoStackFunc, ctx, message, err, fields)
	if fn == nil {
		panic("unexpected Logger.ErrorNoStack call")
	}
	fn(ctx, message, err, fields...)
}

func (l *Logger) SetLogSink(sink logger.LogSinkFunc) {
	l.mu.Lock()
	l.SetLogSinkCalls++
	l.SetLogSinkSink = sink
	fn := l.SetLogSinkFunc
	l.mu.Unlock()
	if fn == nil {
		panic("unexpected Logger.SetLogSink call")
	}
	fn(sink)
}

func (l *Logger) SetContextFieldsProvider(provider logger.ContextFieldsProviderFunc) {
	l.mu.Lock()
	l.SetContextFieldsProviderCalls++
	l.SetContextFieldsProviderProvider = provider
	fn := l.SetContextFieldsProviderFunc
	l.mu.Unlock()
	if fn == nil {
		panic("unexpected Logger.SetContextFieldsProvider call")
	}
	fn(provider)
}

func (l *Logger) Sync() error {
	l.mu.Lock()
	l.SyncCalls++
	fn := l.SyncFunc
	l.mu.Unlock()
	if fn == nil {
		panic("unexpected Logger.Sync call")
	}
	return fn()
}

func (l *Logger) recordLogCall(calls *[]LogCall, fn func(context.Context, string, ...logger.Fields), ctx context.Context, message string, fields []logger.Fields) func(context.Context, string, ...logger.Fields) {
	l.mu.Lock()
	defer l.mu.Unlock()
	*calls = append(*calls, LogCall{Context: ctx, Message: message, Fields: append([]logger.Fields(nil), fields...)})
	return fn
}

func (l *Logger) recordErrorCall(calls *[]ErrorCall, fn func(context.Context, string, error, ...logger.Fields), ctx context.Context, message string, err error, fields []logger.Fields) func(context.Context, string, error, ...logger.Fields) {
	l.mu.Lock()
	defer l.mu.Unlock()
	*calls = append(*calls, ErrorCall{Context: ctx, Message: message, Err: err, Fields: append([]logger.Fields(nil), fields...)})
	return fn
}

var _ logger.Logger = (*Logger)(nil)
