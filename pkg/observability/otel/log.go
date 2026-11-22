package otel

import (
	"context"
	"fmt"
	"time"

	"github.com/eannchen/go-backend-architecture/pkg/observability"

	"go.opentelemetry.io/otel/attribute"
	otellog "go.opentelemetry.io/otel/log"
)

type otelLogEmitter struct {
	logger otellog.Logger
}

func NewOtelLogEmitter(loggerProvider otellog.LoggerProvider, serviceName string) observability.LogEmitter {
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
	record.SetBody(attribute.StringValue(message))
	record.AddAttributes(toLogAttributes(fields)...)
	// Passing the active span context lets the OTel SDK attach its trace and span
	// IDs to the log record. Observability backends can then link this log to the
	// exported span without reading the original traceparent wire value.
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

func toLogAttributes(fields observability.Fields) []attribute.KeyValue {
	out := make([]attribute.KeyValue, 0, len(fields))
	for key, value := range fields {
		out = append(out, attribute.KeyValue{
			Key:   attribute.Key(key),
			Value: toLogValue(value),
		})
	}
	return out
}

func toLogValue(v any) attribute.Value {
	switch t := v.(type) {
	case string:
		return attribute.StringValue(t)
	case bool:
		return attribute.BoolValue(t)
	case int:
		return attribute.IntValue(t)
	case int32:
		return attribute.Int64Value(int64(t))
	case int64:
		return attribute.Int64Value(t)
	case uint:
		return attribute.Int64Value(int64(t))
	case uint32:
		return attribute.Int64Value(int64(t))
	case uint64:
		return attribute.Int64Value(int64(t))
	case float32:
		return attribute.Float64Value(float64(t))
	case float64:
		return attribute.Float64Value(t)
	case time.Time:
		return attribute.StringValue(t.Format(time.RFC3339Nano))
	case error:
		return attribute.StringValue(t.Error())
	default:
		return attribute.StringValue(fmt.Sprintf("%v", v))
	}
}
