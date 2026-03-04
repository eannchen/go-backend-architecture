package logger

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"vocynex-api/internal/infra/config"
	"vocynex-api/internal/infra/observability"
)

type zapLogger struct {
	base      *zap.Logger
	otelLevel zapcore.Level
	emitSink  LogSinkFunc
}

func NewZap(cfg config.LogConfig) (Logger, error) {
	level, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}
	otelLevel, err := zapcore.ParseLevel(cfg.OTELevel)
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

	return &zapLogger{
		base:      logger,
		otelLevel: otelLevel,
		emitSink:  func(context.Context, string, string, ...Field) {},
	}, nil
}

func (l *zapLogger) Debug(ctx context.Context, message string, fields ...Field) {
	l.base.With(contextFields(ctx)...).Debug(message, toZapFields(fields)...)
	l.emitOTelLog(ctx, zapcore.DebugLevel, "debug", message, fields...)
}

func (l *zapLogger) Info(ctx context.Context, message string, fields ...Field) {
	l.base.With(contextFields(ctx)...).Info(message, toZapFields(fields)...)
	l.emitOTelLog(ctx, zapcore.InfoLevel, "info", message, fields...)
}

func (l *zapLogger) Warn(ctx context.Context, message string, fields ...Field) {
	l.base.With(contextFields(ctx)...).Warn(message, toZapFields(fields)...)
	l.emitOTelLog(ctx, zapcore.WarnLevel, "warn", message, fields...)
}

func (l *zapLogger) Error(ctx context.Context, message string, err error, fields ...Field) {
	zf := append(fields, Field{Key: "error", Value: err})
	l.base.With(contextFields(ctx)...).Error(message, toZapFields(zf)...)
	l.emitOTelLog(ctx, zapcore.ErrorLevel, "error", message, zf...)
}

func (l *zapLogger) SetLogSink(sink LogSinkFunc) {
	if sink == nil {
		l.emitSink = func(context.Context, string, string, ...Field) {}
		return
	}
	l.emitSink = sink
}

func (l *zapLogger) Sync() error {
	return l.base.Sync()
}

func contextFields(ctx context.Context) []zap.Field {
	fields := make([]zap.Field, 0, 3)

	if requestID := observability.RequestIDFromContext(ctx); requestID != "" {
		fields = append(fields, zap.String("request_id", requestID))
	}
	traceID, spanID := observability.TraceFromContext(ctx)
	if traceID != "" {
		fields = append(fields, zap.String("trace_id", traceID))
	}
	if spanID != "" {
		fields = append(fields, zap.String("span_id", spanID))
	}

	return fields
}

func toZapFields(fields []Field) []zap.Field {
	out := make([]zap.Field, 0, len(fields))
	for _, f := range fields {
		out = append(out, zap.Any(f.Key, f.Value))
	}
	return out
}

func (l *zapLogger) emitOTelLog(ctx context.Context, level zapcore.Level, severityText, message string, fields ...Field) {
	if level < l.otelLevel {
		return
	}
	l.emitSink(ctx, severityText, message, fields...)
}
