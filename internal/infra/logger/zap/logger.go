package zaplogger

import (
	"context"
	"fmt"
	"runtime"

	uzap "go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"vocynex-api/internal/infra/config"
	"vocynex-api/internal/infra/logger"
)

type impl struct {
	base                  *uzap.Logger
	sinkLevel             zapcore.Level
	emitSink              logger.LogSinkFunc
	contextFieldsProvider logger.ContextFieldsProviderFunc
}

func New(cfg config.LogConfig) (logger.Logger, error) {
	level, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}
	sinkLevel, err := zapcore.ParseLevel(cfg.OTELevel)
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
	base = base.WithOptions(uzap.AddCaller(), uzap.AddCallerSkip(1))

	return &impl{
		base:                  base,
		sinkLevel:             sinkLevel,
		emitSink:              func(context.Context, logger.Severity, string, ...logger.Field) {},
		contextFieldsProvider: func(context.Context) []logger.Field { return nil },
	}, nil
}

func (l *impl) Debug(ctx context.Context, message string, fieldSets ...[]logger.Field) {
	fields := flattenFieldSets(fieldSets...)
	contextFields := l.contextFieldsProvider(ctx)
	l.base.With(buildZapFields(contextFields)...).Debug(message, buildZapFields(fields)...)
	l.emitSinkLog(ctx, zapcore.DebugLevel, logger.SeverityDebug, message, fields, contextFields)
}

func (l *impl) Info(ctx context.Context, message string, fieldSets ...[]logger.Field) {
	fields := flattenFieldSets(fieldSets...)
	contextFields := l.contextFieldsProvider(ctx)
	l.base.With(buildZapFields(contextFields)...).Info(message, buildZapFields(fields)...)
	l.emitSinkLog(ctx, zapcore.InfoLevel, logger.SeverityInfo, message, fields, contextFields)
}

func (l *impl) Warn(ctx context.Context, message string, fieldSets ...[]logger.Field) {
	fields := flattenFieldSets(fieldSets...)
	contextFields := l.contextFieldsProvider(ctx)
	l.base.With(buildZapFields(contextFields)...).Warn(message, buildZapFields(fields)...)
	l.emitSinkLog(ctx, zapcore.WarnLevel, logger.SeverityWarn, message, fields, contextFields)
}

func (l *impl) Error(ctx context.Context, message string, err error, fieldSets ...[]logger.Field) {
	fields := flattenFieldSets(fieldSets...)
	fields = append(fields, logger.FieldOf("error", err))
	contextFields := l.contextFieldsProvider(ctx)
	l.base.With(buildZapFields(contextFields)...).Error(message, buildZapFields(fields)...)
	l.emitSinkLog(ctx, zapcore.ErrorLevel, logger.SeverityError, message, fields, contextFields)
}

func (l *impl) SetLogSink(sink logger.LogSinkFunc) {
	if sink == nil {
		l.emitSink = func(context.Context, logger.Severity, string, ...logger.Field) {}
		return
	}
	l.emitSink = sink
}

func (l *impl) SetContextFieldsProvider(provider logger.ContextFieldsProviderFunc) {
	if provider == nil {
		l.contextFieldsProvider = func(context.Context) []logger.Field { return nil }
		return
	}
	l.contextFieldsProvider = provider
}

func (l *impl) Sync() error {
	return l.base.Sync()
}

func (l *impl) emitSinkLog(ctx context.Context, level zapcore.Level, severity logger.Severity, message string, fields []logger.Field, contextFields []logger.Field) {
	if level < l.sinkLevel {
		return
	}
	callerFields := buildCallerSinkFields()
	out := make([]logger.Field, 0, len(fields)+len(contextFields)+len(callerFields))
	out = append(out, fields...)
	out = append(out, contextFields...)
	out = append(out, callerFields...)
	l.emitSink(ctx, severity, message, out...)
}

func buildZapFields(fields []logger.Field) []uzap.Field {
	out := make([]uzap.Field, 0, len(fields))
	for _, f := range fields {
		out = append(out, uzap.Any(f.Key, f.Value))
	}
	return out
}

func flattenFieldSets(fieldSets ...[]logger.Field) []logger.Field {
	total := 0
	for _, set := range fieldSets {
		total += len(set)
	}
	out := make([]logger.Field, 0, total)
	for _, set := range fieldSets {
		out = append(out, set...)
	}
	return out
}

func buildCallerSinkFields() []logger.Field {
	pc, file, line, ok := runtime.Caller(3)
	if !ok {
		return nil
	}
	fn := runtime.FuncForPC(pc)
	funcName := ""
	if fn != nil {
		funcName = fn.Name()
	}
	return []logger.Field{
		logger.FieldOf("code.location", fmt.Sprintf("%s:%d", file, line)),
		logger.FieldOf("code.function", funcName),
	}
}
