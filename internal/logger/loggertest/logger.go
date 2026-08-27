package loggertest

import (
	"context"

	"github.com/eannchen/go-backend-architecture/internal/logger"
)

// Logger records log calls while satisfying the project logger contract.
type Logger struct {
	InfoCalls         int
	Infos             []Record
	WarnCalls         int
	Warns             []Record
	ErrorCalls        int
	Errors            []ErrorRecord
	ErrorNoStackCalls int
	ErrorsNoStack     []ErrorRecord
}

type Record struct {
	Message string
	Fields  []logger.Fields
}

type ErrorRecord struct {
	Message string
	Err     error
	Fields  []logger.Fields
}

func (l *Logger) Debug(context.Context, string, ...logger.Fields) {}
func (l *Logger) Info(_ context.Context, message string, fields ...logger.Fields) {
	l.InfoCalls++
	l.Infos = append(l.Infos, Record{Message: message, Fields: fields})
}

func (l *Logger) Warn(_ context.Context, message string, fields ...logger.Fields) {
	l.WarnCalls++
	l.Warns = append(l.Warns, Record{Message: message, Fields: fields})
}

func (l *Logger) Error(_ context.Context, message string, err error, fields ...logger.Fields) {
	l.ErrorCalls++
	l.Errors = append(l.Errors, ErrorRecord{Message: message, Err: err, Fields: fields})
}
func (l *Logger) ErrorNoStack(_ context.Context, message string, err error, fields ...logger.Fields) {
	l.ErrorNoStackCalls++
	l.ErrorsNoStack = append(l.ErrorsNoStack, ErrorRecord{Message: message, Err: err, Fields: fields})
}
func (l *Logger) SetLogSink(logger.LogSinkFunc) {}
func (l *Logger) SetContextFieldsProvider(logger.ContextFieldsProviderFunc) {
}
func (l *Logger) Sync() error { return nil }

var _ logger.Logger = (*Logger)(nil)
