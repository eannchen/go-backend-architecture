package loggertest

import (
	"context"

	"github.com/eannchen/go-backend-architecture/internal/logger"
)

// Logger records log calls while satisfying the project logger contract.
type Logger struct {
	WarnCalls int
	Warns     []Record
}

type Record struct {
	Message string
	Fields  []logger.Fields
}

func (l *Logger) Debug(context.Context, string, ...logger.Fields) {}
func (l *Logger) Info(context.Context, string, ...logger.Fields)  {}

func (l *Logger) Warn(_ context.Context, message string, fields ...logger.Fields) {
	l.WarnCalls++
	l.Warns = append(l.Warns, Record{Message: message, Fields: fields})
}

func (l *Logger) Error(context.Context, string, error, ...logger.Fields) {}
func (l *Logger) ErrorNoStack(context.Context, string, error, ...logger.Fields) {
}
func (l *Logger) SetLogSink(logger.LogSinkFunc) {}
func (l *Logger) SetContextFieldsProvider(logger.ContextFieldsProviderFunc) {
}
func (l *Logger) Sync() error { return nil }

var _ logger.Logger = (*Logger)(nil)
