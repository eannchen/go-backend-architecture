package observabilitytest

import (
	"context"
	"sync"

	"github.com/eannchen/go-backend-architecture/internal/observability"
)

// MetricSample records one metric value and its normalized fields.
type MetricSample struct {
	Value  int64
	Fields observability.Fields
}

// HistogramSample records one histogram value and its normalized fields.
type HistogramSample struct {
	Value  float64
	Fields observability.Fields
}

// RecordingMeter is a concurrency-safe behavioral double for observability.Meter.
type RecordingMeter struct {
	mu             sync.Mutex
	counters       map[string][]MetricSample
	upDownCounters map[string][]MetricSample
	histograms     map[string][]HistogramSample
}

func NewRecordingMeter() *RecordingMeter {
	return &RecordingMeter{
		counters:       make(map[string][]MetricSample),
		upDownCounters: make(map[string][]MetricSample),
		histograms:     make(map[string][]HistogramSample),
	}
}

func (m *RecordingMeter) Counter(name string, _ ...observability.MetricOption) observability.Counter {
	return recordingCounter{meter: m, name: name, samples: m.counters}
}

func (m *RecordingMeter) UpDownCounter(name string, _ ...observability.MetricOption) observability.UpDownCounter {
	return recordingCounter{meter: m, name: name, samples: m.upDownCounters}
}

func (m *RecordingMeter) Histogram(name string, _ ...observability.MetricOption) observability.Histogram {
	return recordingHistogram{meter: m, name: name}
}

func (m *RecordingMeter) CounterSamples(name string) []MetricSample {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]MetricSample(nil), m.counters[name]...)
}

func (m *RecordingMeter) UpDownCounterSamples(name string) []MetricSample {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]MetricSample(nil), m.upDownCounters[name]...)
}

func (m *RecordingMeter) HistogramSamples(name string) []HistogramSample {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]HistogramSample(nil), m.histograms[name]...)
}

type recordingCounter struct {
	meter   *RecordingMeter
	name    string
	samples map[string][]MetricSample
}

func (c recordingCounter) Add(_ context.Context, value int64, fields ...observability.Fields) {
	c.meter.mu.Lock()
	defer c.meter.mu.Unlock()
	c.samples[c.name] = append(c.samples[c.name], MetricSample{
		Value:  value,
		Fields: observability.OptionalFields(fields...),
	})
}

type recordingHistogram struct {
	meter *RecordingMeter
	name  string
}

func (h recordingHistogram) Record(_ context.Context, value float64, fields ...observability.Fields) {
	h.meter.mu.Lock()
	defer h.meter.mu.Unlock()
	h.meter.histograms[h.name] = append(h.meter.histograms[h.name], HistogramSample{
		Value:  value,
		Fields: observability.OptionalFields(fields...),
	})
}

var _ observability.Meter = (*RecordingMeter)(nil)
