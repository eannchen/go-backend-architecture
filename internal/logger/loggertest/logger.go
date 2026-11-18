package loggertest

import (
	"context"
	"sync"

	"github.com/eannchen/go-backend-architecture/internal/logger"
)

type LogCall struct {
	Context context.Context
	Message string
	Fields  []logger.Fields
}

type ErrorCall struct {
	Context context.Context
	Message string
	Err     error
	Fields  []logger.Fields
}

// Logger is a configurable double for its parent package's Logger contract.
// Configure callbacks before use and inspect calls after concurrent work finishes.
type Logger struct {
	mu                            sync.Mutex
	DebugFunc                     func(ctx context.Context, message string, fields ...logger.Fields)
	DebugCalls                    []LogCall
	InfoFunc                      func(ctx context.Context, message string, fields ...logger.Fields)
	InfoCalls                     []LogCall
	WarnFunc                      func(ctx context.Context, message string, fields ...logger.Fields)
	WarnCalls                     []LogCall
	ErrorFunc                     func(ctx context.Context, message string, err error, fields ...logger.Fields)
	ErrorCalls                    []ErrorCall
	ErrorNoStackFunc              func(ctx context.Context, message string, err error, fields ...logger.Fields)
	ErrorNoStackCalls             []ErrorCall
	SetLogSinkFunc                func(sink logger.LogSinkFunc)
	SetLogSinkCalls               []LoggerSetLogSinkCall
	SetContextFieldsProviderFunc  func(provider logger.ContextFieldsProviderFunc)
	SetContextFieldsProviderCalls []LoggerSetContextFieldsProviderCall
	SyncFunc                      func() error
	SyncCalls                     []LoggerSyncCall
}

func (d *Logger) Debug(ctx context.Context, message string, fields ...logger.Fields) {
	d.mu.Lock()
	d.DebugCalls = append(d.DebugCalls, LogCall{Context: ctx, Message: message, Fields: append([]logger.Fields(nil), fields...)})
	fn := d.DebugFunc
	d.mu.Unlock()
	if fn == nil {
		panic("unexpected Logger.Debug call")
	}
	fn(ctx, message, fields...)
}

func (d *Logger) Info(ctx context.Context, message string, fields ...logger.Fields) {
	d.mu.Lock()
	d.InfoCalls = append(d.InfoCalls, LogCall{Context: ctx, Message: message, Fields: append([]logger.Fields(nil), fields...)})
	fn := d.InfoFunc
	d.mu.Unlock()
	if fn == nil {
		panic("unexpected Logger.Info call")
	}
	fn(ctx, message, fields...)
}

func (d *Logger) Warn(ctx context.Context, message string, fields ...logger.Fields) {
	d.mu.Lock()
	d.WarnCalls = append(d.WarnCalls, LogCall{Context: ctx, Message: message, Fields: append([]logger.Fields(nil), fields...)})
	fn := d.WarnFunc
	d.mu.Unlock()
	if fn == nil {
		panic("unexpected Logger.Warn call")
	}
	fn(ctx, message, fields...)
}

func (d *Logger) Error(ctx context.Context, message string, err error, fields ...logger.Fields) {
	d.mu.Lock()
	d.ErrorCalls = append(d.ErrorCalls, ErrorCall{Context: ctx, Message: message, Err: err, Fields: append([]logger.Fields(nil), fields...)})
	fn := d.ErrorFunc
	d.mu.Unlock()
	if fn == nil {
		panic("unexpected Logger.Error call")
	}
	fn(ctx, message, err, fields...)
}

func (d *Logger) ErrorNoStack(ctx context.Context, message string, err error, fields ...logger.Fields) {
	d.mu.Lock()
	d.ErrorNoStackCalls = append(d.ErrorNoStackCalls, ErrorCall{Context: ctx, Message: message, Err: err, Fields: append([]logger.Fields(nil), fields...)})
	fn := d.ErrorNoStackFunc
	d.mu.Unlock()
	if fn == nil {
		panic("unexpected Logger.ErrorNoStack call")
	}
	fn(ctx, message, err, fields...)
}

// LoggerSetLogSinkCall records one Logger.SetLogSink invocation.
type LoggerSetLogSinkCall struct {
	Sink logger.LogSinkFunc
}

func (d *Logger) SetLogSink(sink logger.LogSinkFunc) {
	d.mu.Lock()
	d.SetLogSinkCalls = append(d.SetLogSinkCalls, LoggerSetLogSinkCall{Sink: sink})
	fn := d.SetLogSinkFunc
	d.mu.Unlock()
	if fn == nil {
		panic("unexpected Logger.SetLogSink call")
	}
	fn(sink)
}

// LoggerSetContextFieldsProviderCall records one Logger.SetContextFieldsProvider invocation.
type LoggerSetContextFieldsProviderCall struct {
	Provider logger.ContextFieldsProviderFunc
}

func (d *Logger) SetContextFieldsProvider(provider logger.ContextFieldsProviderFunc) {
	d.mu.Lock()
	d.SetContextFieldsProviderCalls = append(d.SetContextFieldsProviderCalls, LoggerSetContextFieldsProviderCall{Provider: provider})
	fn := d.SetContextFieldsProviderFunc
	d.mu.Unlock()
	if fn == nil {
		panic("unexpected Logger.SetContextFieldsProvider call")
	}
	fn(provider)
}

// LoggerSyncCall records one Logger.Sync invocation.
type LoggerSyncCall struct {
}

func (d *Logger) Sync() error {
	d.mu.Lock()
	d.SyncCalls = append(d.SyncCalls, LoggerSyncCall{})
	fn := d.SyncFunc
	d.mu.Unlock()
	if fn == nil {
		panic("unexpected Logger.Sync call")
	}
	return fn()
}
