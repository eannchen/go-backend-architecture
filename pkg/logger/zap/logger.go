package zaplogger

import (
	"context"
	"fmt"
	"runtime"

	uzap "go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/eannchen/go-backend-architecture/pkg/logger"
)

// Config controls local output and the optional secondary sink.
type Config struct {
	// Level is the minimum level written by Zap. An empty value means info.
	Level string
	// SinkLevel is the minimum level forwarded to SetLogSink's callback.
	// An empty value means info, independently of Level.
	SinkLevel string
	// Development enables Zap's development formatting and diagnostics.
	Development bool
}

// Logger implements structured logging using Zap. Configure optional callbacks
// before logging starts; app composition owns sink integration and shutdown.
type Logger struct {
	base                  *uzap.Logger
	sinkLevel             zapcore.Level
	emitSink              logger.LogSinkFunc
	contextFieldsProvider logger.ContextFieldsProviderFunc
}

// New builds a logger from explicit configuration.
func New(cfg Config) (*Logger, error) {
	level, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}
	sinkLevel, err := zapcore.ParseLevel(cfg.SinkLevel)
	if err != nil {
		return nil, err
	}

	zapCfg := uzap.NewProductionConfig()
	if cfg.Development {
		zapCfg = uzap.NewDevelopmentConfig()
	}
	zapCfg.Level.SetLevel(level)

	base, err := zapCfg.Build()
	if err != nil {
		return nil, err
	}
	return newLogger(base, sinkLevel), nil
}

func newLogger(base *uzap.Logger, sinkLevel zapcore.Level) *Logger {
	return &Logger{
		base:                  base.WithOptions(uzap.AddCaller(), uzap.AddCallerSkip(1)),
		sinkLevel:             sinkLevel,
		emitSink:              func(context.Context, logger.Severity, string, ...logger.Fields) {},
		contextFieldsProvider: func(context.Context) logger.Fields { return nil },
	}
}

func (l *Logger) Debug(ctx context.Context, message string, optionalFields ...logger.Fields) {
	fields := logger.OptionalFields(optionalFields...)
	contextFields := l.contextFieldsProvider(ctx)
	l.base.With(buildZapFields(contextFields)...).Debug(message, buildZapFields(fields)...)
	l.emitSinkLog(ctx, zapcore.DebugLevel, logger.SeverityDebug, message, fields, contextFields, 3)
}

func (l *Logger) Info(ctx context.Context, message string, optionalFields ...logger.Fields) {
	fields := logger.OptionalFields(optionalFields...)
	contextFields := l.contextFieldsProvider(ctx)
	l.base.With(buildZapFields(contextFields)...).Info(message, buildZapFields(fields)...)
	l.emitSinkLog(ctx, zapcore.InfoLevel, logger.SeverityInfo, message, fields, contextFields, 3)
}

func (l *Logger) Warn(ctx context.Context, message string, optionalFields ...logger.Fields) {
	fields := logger.OptionalFields(optionalFields...)
	contextFields := l.contextFieldsProvider(ctx)
	l.base.With(buildZapFields(contextFields)...).Warn(message, buildZapFields(fields)...)
	l.emitSinkLog(ctx, zapcore.WarnLevel, logger.SeverityWarn, message, fields, contextFields, 3)
}

func (l *Logger) Error(ctx context.Context, message string, err error, optionalFields ...logger.Fields) {
	l.logError(ctx, message, err, true, optionalFields...)
}

func (l *Logger) ErrorNoStack(ctx context.Context, message string, err error, optionalFields ...logger.Fields) {
	l.logError(ctx, message, err, false, optionalFields...)
}

func (l *Logger) logError(ctx context.Context, message string, err error, stacktrace bool, optionalFields ...logger.Fields) {
	fields := logger.CloneFields(logger.OptionalFields(optionalFields...))
	if fields == nil {
		fields = logger.Fields{}
	}
	fields["error"] = err
	contextFields := l.contextFieldsProvider(ctx)
	base := l.base
	// Error and ErrorNoStack share this helper, adding one frame beyond the
	// public logging method. Skip it so caller metadata still names application code.
	base = base.WithOptions(uzap.AddCallerSkip(1))
	if !stacktrace {
		base = base.WithOptions(uzap.AddStacktrace(zapcore.PanicLevel))
	}
	base.With(buildZapFields(contextFields)...).Error(message, buildZapFields(fields)...)
	l.emitSinkLog(ctx, zapcore.ErrorLevel, logger.SeverityError, message, fields, contextFields, 4)
}

func (l *Logger) SetLogSink(sink logger.LogSinkFunc) {
	if sink == nil {
		l.emitSink = func(context.Context, logger.Severity, string, ...logger.Fields) {}
		return
	}
	l.emitSink = sink
}

func (l *Logger) SetContextFieldsProvider(provider logger.ContextFieldsProviderFunc) {
	if provider == nil {
		l.contextFieldsProvider = func(context.Context) logger.Fields { return nil }
		return
	}
	l.contextFieldsProvider = provider
}

func (l *Logger) Sync() error {
	return l.base.Sync()
}

func (l *Logger) emitSinkLog(ctx context.Context, level zapcore.Level, severity logger.Severity, message string, fields, contextFields logger.Fields, callerSkip int) {
	if level < l.sinkLevel {
		return
	}
	callerFields := buildCallerSinkFields(callerSkip)
	// Explicit fields match Zap's per-call behavior and override context fields
	// when both sources use the same key.
	out := logger.MergeFields(contextFields, fields, callerFields)
	l.emitSink(ctx, severity, message, out)
}

func buildZapFields(fields logger.Fields) []uzap.Field {
	out := make([]uzap.Field, 0, len(fields))
	for key, value := range fields {
		out = append(out, uzap.Any(key, value))
	}
	return out
}

func buildCallerSinkFields(skip int) logger.Fields {
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return nil
	}
	fn := runtime.FuncForPC(pc)
	funcName := ""
	if fn != nil {
		funcName = fn.Name()
	}
	return logger.FromPairs(
		"code.location", fmt.Sprintf("%s:%d", file, line),
		"code.function", funcName,
	)
}
