package otel

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/eannchen/go-backend-architecture/internal/observability"
)

type meter struct {
	meter metric.Meter

	// Instrument creation can happen from many goroutines; cache by name/type to
	// avoid repeated registration overhead on hot paths.
	counters       sync.Map
	upDownCounters sync.Map
	histograms     sync.Map
}

func NewMeter(meterProvider metric.MeterProvider, serviceName string) observability.Meter {
	return &meter{
		meter: meterProvider.Meter(metricScopeName(serviceName)),
	}
}

func metricScopeName(serviceName string) string {
	return serviceName + "/metrics"
}

func (m *meter) Counter(name string, options ...observability.MetricOption) observability.Counter {
	if name == "" {
		return observability.NoopMeter{}.Counter(name, options...)
	}
	if cached, ok := m.counters.Load(name); ok {
		return cached.(observability.Counter)
	}

	counter, err := m.meter.Int64Counter(name, toCounterOptions(options...)...)
	if err != nil {
		return observability.NoopMeter{}.Counter(name, options...)
	}
	inst := &otelCounter{instrument: counter}
	actual, _ := m.counters.LoadOrStore(name, inst)
	return actual.(observability.Counter)
}

func (m *meter) UpDownCounter(name string, options ...observability.MetricOption) observability.UpDownCounter {
	if name == "" {
		return observability.NoopMeter{}.UpDownCounter(name, options...)
	}
	if cached, ok := m.upDownCounters.Load(name); ok {
		return cached.(observability.UpDownCounter)
	}

	counter, err := m.meter.Int64UpDownCounter(name, toUpDownCounterOptions(options...)...)
	if err != nil {
		return observability.NoopMeter{}.UpDownCounter(name, options...)
	}
	inst := &otelUpDownCounter{instrument: counter}
	actual, _ := m.upDownCounters.LoadOrStore(name, inst)
	return actual.(observability.UpDownCounter)
}

func (m *meter) Histogram(name string, options ...observability.MetricOption) observability.Histogram {
	if name == "" {
		return observability.NoopMeter{}.Histogram(name, options...)
	}
	if cached, ok := m.histograms.Load(name); ok {
		return cached.(observability.Histogram)
	}

	histogram, err := m.meter.Float64Histogram(name, toHistogramOptions(options...)...)
	if err != nil {
		return observability.NoopMeter{}.Histogram(name, options...)
	}
	inst := &otelHistogram{instrument: histogram}
	actual, _ := m.histograms.LoadOrStore(name, inst)
	return actual.(observability.Histogram)
}

type otelCounter struct {
	instrument metric.Int64Counter
}

func (c *otelCounter) Add(ctx context.Context, value int64, optionalFields ...observability.Fields) {
	c.instrument.Add(ctx, value, metric.WithAttributes(toMetricAttributes(observability.OptionalFields(optionalFields...))...))
}

type otelUpDownCounter struct {
	instrument metric.Int64UpDownCounter
}

func (c *otelUpDownCounter) Add(ctx context.Context, value int64, optionalFields ...observability.Fields) {
	c.instrument.Add(ctx, value, metric.WithAttributes(toMetricAttributes(observability.OptionalFields(optionalFields...))...))
}

type otelHistogram struct {
	instrument metric.Float64Histogram
}

func (h *otelHistogram) Record(ctx context.Context, value float64, optionalFields ...observability.Fields) {
	h.instrument.Record(ctx, value, metric.WithAttributes(toMetricAttributes(observability.OptionalFields(optionalFields...))...))
}

func toMetricAttributes(fields observability.Fields) []attribute.KeyValue {
	out := make([]attribute.KeyValue, 0, len(fields))
	for key, value := range fields {
		out = append(out, toTraceAttribute(key, value))
	}
	return out
}

func toCounterOptions(options ...observability.MetricOption) []metric.Int64CounterOption {
	if len(options) == 0 {
		return nil
	}
	o := options[0]
	out := make([]metric.Int64CounterOption, 0, 2)
	if o.Description != "" {
		out = append(out, metric.WithDescription(o.Description))
	}
	if o.Unit != "" {
		out = append(out, metric.WithUnit(o.Unit))
	}
	return out
}

func toUpDownCounterOptions(options ...observability.MetricOption) []metric.Int64UpDownCounterOption {
	if len(options) == 0 {
		return nil
	}
	o := options[0]
	out := make([]metric.Int64UpDownCounterOption, 0, 2)
	if o.Description != "" {
		out = append(out, metric.WithDescription(o.Description))
	}
	if o.Unit != "" {
		out = append(out, metric.WithUnit(o.Unit))
	}
	return out
}

func toHistogramOptions(options ...observability.MetricOption) []metric.Float64HistogramOption {
	if len(options) == 0 {
		return nil
	}
	o := options[0]
	out := make([]metric.Float64HistogramOption, 0, 2)
	if o.Description != "" {
		out = append(out, metric.WithDescription(o.Description))
	}
	if o.Unit != "" {
		out = append(out, metric.WithUnit(o.Unit))
	}
	return out
}
