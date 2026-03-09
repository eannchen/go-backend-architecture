package observability

import "context"

// MetricOption configures metric instruments.
type MetricOption struct {
	Description string
	Unit        string
}

// Counter records monotonically increasing integer values.
type Counter interface {
	Add(ctx context.Context, value int64, fields ...Fields)
}

// UpDownCounter records integer values that can increase or decrease.
type UpDownCounter interface {
	Add(ctx context.Context, value int64, fields ...Fields)
}

// Histogram records distribution samples.
type Histogram interface {
	Record(ctx context.Context, value float64, fields ...Fields)
}

// Meter creates and owns metric instruments for app layers.
type Meter interface {
	Counter(name string, options ...MetricOption) Counter
	UpDownCounter(name string, options ...MetricOption) UpDownCounter
	Histogram(name string, options ...MetricOption) Histogram
}

// NoopMeter is a no-op implementation of Meter.
type NoopMeter struct{}

type noopCounter struct{}
type noopUpDownCounter struct{}
type noopHistogram struct{}

func (NoopMeter) Counter(string, ...MetricOption) Counter { return noopCounter{} }

func (NoopMeter) UpDownCounter(string, ...MetricOption) UpDownCounter { return noopUpDownCounter{} }

func (NoopMeter) Histogram(string, ...MetricOption) Histogram { return noopHistogram{} }

func (noopCounter) Add(context.Context, int64, ...Fields) {}

func (noopUpDownCounter) Add(context.Context, int64, ...Fields) {}

func (noopHistogram) Record(context.Context, float64, ...Fields) {}
