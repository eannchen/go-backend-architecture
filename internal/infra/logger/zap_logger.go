package logger

import (
	"context"
	"fmt"
	"runtime"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"vocynex-api/internal/infra/config"
)

type zapLogger struct {
	base                  *zap.Logger
	sinkLevel             zapcore.Level
	emitSink              LogSinkFunc
	contextFieldsProvider ContextFieldsProviderFunc
}

func NewZap(cfg config.LogConfig) (Logger, error) {
	level, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}
	sinkLevel, err := zapcore.ParseLevel(cfg.OTELevel)
	if err != nil {
		return nil, err
	}

	zapCfg := zap.NewProductionConfig()
	if cfg.Development {
		zapCfg = zap.NewDevelopmentConfig()
	}
	zapCfg.Level.SetLevel(level)

	logger, err := zapCfg.Build()
	if err != nil {
		return nil, err
	}
	// Skip one frame to show real caller.
	logger = logger.WithOptions(zap.AddCaller(), zap.AddCallerSkip(1))

	return &zapLogger{
		base:                  logger,
		sinkLevel:             sinkLevel,
		emitSink:              func(context.Context, Severity, string, ...Field) {},
		contextFieldsProvider: func(context.Context) []Field { return nil },
	}, nil
}

func (l *zapLogger) Debug(ctx context.Context, message string, fieldSets ...[]Field) {
	fields := flattenFieldSets(fieldSets...)
	contextFields := l.contextFieldsProvider(ctx)
	l.base.With(buildZapFields(contextFields)...).Debug(message, buildZapFields(fields)...)
	l.emitSinkLog(ctx, zapcore.DebugLevel, SeverityDebug, message, fields, contextFields)
}

func (l *zapLogger) Info(ctx context.Context, message string, fieldSets ...[]Field) {
	fields := flattenFieldSets(fieldSets...)
	contextFields := l.contextFieldsProvider(ctx)
	l.base.With(buildZapFields(contextFields)...).Info(message, buildZapFields(fields)...)
	l.emitSinkLog(ctx, zapcore.InfoLevel, SeverityInfo, message, fields, contextFields)
}

func (l *zapLogger) Warn(ctx context.Context, message string, fieldSets ...[]Field) {
	fields := flattenFieldSets(fieldSets...)
	contextFields := l.contextFieldsProvider(ctx)
	l.base.With(buildZapFields(contextFields)...).Warn(message, buildZapFields(fields)...)
	l.emitSinkLog(ctx, zapcore.WarnLevel, SeverityWarn, message, fields, contextFields)
}

func (l *zapLogger) Error(ctx context.Context, message string, err error, fieldSets ...[]Field) {
	fields := flattenFieldSets(fieldSets...)
	fields = append(fields, FieldOf("error", err))
	contextFields := l.contextFieldsProvider(ctx)
	l.base.With(buildZapFields(contextFields)...).Error(message, buildZapFields(fields)...)
	l.emitSinkLog(ctx, zapcore.ErrorLevel, SeverityError, message, fields, contextFields)
}

func (l *zapLogger) SetLogSink(sink LogSinkFunc) {
	if sink == nil {
		l.emitSink = func(context.Context, Severity, string, ...Field) {}
		return
	}
	l.emitSink = sink
}

func (l *zapLogger) SetContextFieldsProvider(provider ContextFieldsProviderFunc) {
	if provider == nil {
		l.contextFieldsProvider = func(context.Context) []Field { return nil }
		return
	}
	l.contextFieldsProvider = provider
}

func (l *zapLogger) emitSinkLog(ctx context.Context, level zapcore.Level, severity Severity, message string, fields []Field, contextFields []Field) {
	if level < l.sinkLevel {
		return
	}
	callerFields := buildCallerSinkFields()
	out := make([]Field, 0, len(fields)+len(contextFields)+len(callerFields))
	out = append(out, fields...)
	out = append(out, contextFields...)
	out = append(out, callerFields...)
	l.emitSink(ctx, severity, message, out...)
}

func (l *zapLogger) Sync() error {
	return l.base.Sync()
}

func buildZapFields(fields []Field) []zap.Field {
	out := make([]zap.Field, 0, len(fields))
	for _, f := range fields {
		out = append(out, zap.Any(f.Key, f.Value))
	}
	return out
}

func flattenFieldSets(fieldSets ...[]Field) []Field {
	total := 0
	for _, set := range fieldSets {
		total += len(set)
	}
	out := make([]Field, 0, total)
	for _, set := range fieldSets {
		out = append(out, set...)
	}
	return out
}

func buildCallerSinkFields() []Field {
	// Stack frame skip:
	// 0 buildCallerSinkFields, 1 emitSinkLog, 2 Debug/Info/Warn/Error, 3 caller.
	pc, file, line, ok := runtime.Caller(3)
	if !ok {
		return nil
	}
	fn := runtime.FuncForPC(pc)
	funcName := ""
	if fn != nil {
		funcName = fn.Name()
	}
	return []Field{
		FieldOf("code.location", fmt.Sprintf("%s:%d", file, line)),
		FieldOf("code.function", funcName),
	}
}
