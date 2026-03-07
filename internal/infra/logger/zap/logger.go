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
		emitSink:              func(context.Context, logger.Severity, string, ...logger.Fields) {},
		contextFieldsProvider: func(context.Context) logger.Fields { return nil },
	}, nil
}

func (l *impl) Debug(ctx context.Context, message string, optionalFields ...logger.Fields) {
	fields := logger.OptionalFields(optionalFields...)
	contextFields := l.contextFieldsProvider(ctx)
	l.base.With(buildZapFields(contextFields)...).Debug(message, buildZapFields(fields)...)
	l.emitSinkLog(ctx, zapcore.DebugLevel, logger.SeverityDebug, message, fields, contextFields)
}

func (l *impl) Info(ctx context.Context, message string, optionalFields ...logger.Fields) {
	fields := logger.OptionalFields(optionalFields...)
	contextFields := l.contextFieldsProvider(ctx)
	l.base.With(buildZapFields(contextFields)...).Info(message, buildZapFields(fields)...)
	l.emitSinkLog(ctx, zapcore.InfoLevel, logger.SeverityInfo, message, fields, contextFields)
}

func (l *impl) Warn(ctx context.Context, message string, optionalFields ...logger.Fields) {
	fields := logger.OptionalFields(optionalFields...)
	contextFields := l.contextFieldsProvider(ctx)
	l.base.With(buildZapFields(contextFields)...).Warn(message, buildZapFields(fields)...)
	l.emitSinkLog(ctx, zapcore.WarnLevel, logger.SeverityWarn, message, fields, contextFields)
}

func (l *impl) Error(ctx context.Context, message string, err error, optionalFields ...logger.Fields) {
	fields := logger.CloneFields(logger.OptionalFields(optionalFields...))
	if fields == nil {
		fields = logger.Fields{}
	}
	fields["error"] = err
	contextFields := l.contextFieldsProvider(ctx)
	l.base.With(buildZapFields(contextFields)...).Error(message, buildZapFields(fields)...)
	l.emitSinkLog(ctx, zapcore.ErrorLevel, logger.SeverityError, message, contextFields, fields)
}

func (l *impl) SetLogSink(sink logger.LogSinkFunc) {
	if sink == nil {
		l.emitSink = func(context.Context, logger.Severity, string, ...logger.Fields) {}
		return
	}
	l.emitSink = sink
}

func (l *impl) SetContextFieldsProvider(provider logger.ContextFieldsProviderFunc) {
	if provider == nil {
		l.contextFieldsProvider = func(context.Context) logger.Fields { return nil }
		return
	}
	l.contextFieldsProvider = provider
}

func (l *impl) Sync() error {
	return l.base.Sync()
}

func (l *impl) emitSinkLog(ctx context.Context, level zapcore.Level, severity logger.Severity, message string, contextFields, fields logger.Fields) {
	if level < l.sinkLevel {
		return
	}
	callerFields := buildCallerSinkFields()
	out := logger.MergeFields(fields, contextFields, callerFields)
	l.emitSink(ctx, severity, message, out)
}

func buildZapFields(fields logger.Fields) []uzap.Field {
	out := make([]uzap.Field, 0, len(fields))
	for key, value := range fields {
		out = append(out, uzap.Any(key, value))
	}
	return out
}

func buildCallerSinkFields() logger.Fields {
	pc, file, line, ok := runtime.Caller(3)
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
