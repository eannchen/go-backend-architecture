package otel

import (
	"context"
	"fmt"
	"go-backend-architecture/internal/observability"
	"time"

	otellog "go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
)

type otelLogEmitter struct {
	logger otellog.Logger
}

func NewOtelLogEmitter(loggerProvider *sdklog.LoggerProvider, serviceName string) observability.LogEmitter {
	return &otelLogEmitter{
		logger: loggerProvider.Logger(logScopeName(serviceName)),
	}
}

func logScopeName(serviceName string) string {
	return serviceName + "/logger"
}

func (e *otelLogEmitter) Emit(ctx context.Context, severity observability.Severity, message string, optionalFields ...observability.Fields) {
	fields := observability.OptionalFields(optionalFields...)
	sev := toOTelSeverity(severity)
	if !e.logger.Enabled(ctx, otellog.EnabledParameters{Severity: sev}) {
		return
	}

	now := time.Now()
	var record otellog.Record
	record.SetTimestamp(now)
	record.SetObservedTimestamp(now)
	record.SetSeverity(sev)
	record.SetSeverityText(severity.String())
	record.SetBody(otellog.StringValue(message))
	record.AddAttributes(toLogAttributes(fields)...)
	e.logger.Emit(ctx, record)
}

func toOTelSeverity(level observability.Severity) otellog.Severity {
	switch level {
	case observability.SeverityDebug:
		return otellog.SeverityDebug
	case observability.SeverityWarn:
		return otellog.SeverityWarn
	case observability.SeverityError:
		return otellog.SeverityError
	default:
		return otellog.SeverityInfo
	}
}

func toLogAttributes(fields observability.Fields) []otellog.KeyValue {
	out := make([]otellog.KeyValue, 0, len(fields))
	for key, value := range fields {
		out = append(out, otellog.KeyValue{
			Key:   key,
			Value: toLogValue(value),
		})
	}
	return out
}

func toLogValue(v any) otellog.Value {
	switch t := v.(type) {
	case string:
		return otellog.StringValue(t)
	case bool:
		return otellog.BoolValue(t)
	case int:
		return otellog.IntValue(t)
	case int32:
		return otellog.Int64Value(int64(t))
	case int64:
		return otellog.Int64Value(t)
	case uint:
		return otellog.Int64Value(int64(t))
	case uint32:
		return otellog.Int64Value(int64(t))
	case uint64:
		return otellog.Int64Value(int64(t))
	case float32:
		return otellog.Float64Value(float64(t))
	case float64:
		return otellog.Float64Value(t)
	case time.Time:
		return otellog.StringValue(t.Format(time.RFC3339Nano))
	case error:
		return otellog.StringValue(t.Error())
	default:
		return otellog.StringValue(fmt.Sprintf("%v", v))
	}
}
