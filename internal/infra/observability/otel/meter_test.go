package otel

import (
	"context"
	"sync"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/eannchen/go-backend-architecture/internal/observability"
)

func TestMetricScopeName(t *testing.T) {
	got := metricScopeName("accounts-api")
	if got != "accounts-api/metrics" {
		t.Fatalf("unexpected scope name: %q", got)
	}
}

func TestMeterCounter_CachesByName(t *testing.T) {
	m := NewMeter(noop.NewMeterProvider(), "svc")

	first := m.Counter("http.server.requests_total")
	second := m.Counter("http.server.requests_total")
	if first != second {
		t.Fatal("expected counter instrument to be cached by name")
	}
}

func TestMeterInstruments_CacheByName(t *testing.T) {
	m := NewMeter(noop.NewMeterProvider(), "svc")

	if first, second := m.UpDownCounter("workers"), m.UpDownCounter("workers"); first != second {
		t.Fatal("expected up/down counter instrument to be cached by name")
	}
	if first, second := m.Histogram("request.duration"), m.Histogram("request.duration"); first != second {
		t.Fatal("expected histogram instrument to be cached by name")
	}
}

func TestMeterInstruments_RecordValuesAttributesAndOptions(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	m := NewMeter(provider, "accounts-api")
	fields := observability.FromPairs("route", "/users")

	m.Counter("requests", observability.MetricOption{Description: "request count", Unit: "{request}"}).Add(context.Background(), 3, fields)
	m.UpDownCounter("workers").Add(context.Background(), -1, fields)
	m.Histogram("request.duration", observability.MetricOption{Unit: "ms"}).Record(context.Background(), 12.5, fields)

	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatalf("collect metrics: %v", err)
	}
	if len(collected.ScopeMetrics) != 1 || collected.ScopeMetrics[0].Scope.Name != "accounts-api/metrics" {
		t.Fatalf("unexpected metric scopes: %+v", collected.ScopeMetrics)
	}

	requestMetric := findMetric(t, collected, "requests")
	if requestMetric.Description != "request count" || requestMetric.Unit != "{request}" {
		t.Fatalf("unexpected counter options: %+v", requestMetric)
	}
	requestSum := requestMetric.Data.(metricdata.Sum[int64])
	if len(requestSum.DataPoints) != 1 || requestSum.DataPoints[0].Value != 3 {
		t.Fatalf("unexpected counter data: %+v", requestSum.DataPoints)
	}
	assertMetricRoute(t, requestSum.DataPoints[0].Attributes)

	workerSum := findMetric(t, collected, "workers").Data.(metricdata.Sum[int64])
	if len(workerSum.DataPoints) != 1 || workerSum.DataPoints[0].Value != -1 || workerSum.IsMonotonic {
		t.Fatalf("unexpected up/down counter data: %+v", workerSum)
	}

	durationMetric := findMetric(t, collected, "request.duration")
	durationHistogram := durationMetric.Data.(metricdata.Histogram[float64])
	if durationMetric.Unit != "ms" || len(durationHistogram.DataPoints) != 1 || durationHistogram.DataPoints[0].Count != 1 || durationHistogram.DataPoints[0].Sum != 12.5 {
		t.Fatalf("unexpected histogram data: metric=%+v data=%+v", durationMetric, durationHistogram)
	}
}

func TestMeterCounter_CachesByNameUnderConcurrency(t *testing.T) {
	m := NewMeter(noop.NewMeterProvider(), "svc")

	const workers = 32
	counters := make([]interface{}, workers)
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		i := i
		go func() {
			defer wg.Done()
			counters[i] = m.Counter("db.query.count")
		}()
	}
	wg.Wait()

	first := counters[0]
	for i := 1; i < len(counters); i++ {
		if counters[i] != first {
			t.Fatalf("expected all goroutines to receive the same cached counter instance, mismatch at index %d", i)
		}
	}
}

func findMetric(t *testing.T, collected metricdata.ResourceMetrics, name string) metricdata.Metrics {
	t.Helper()
	for _, scope := range collected.ScopeMetrics {
		for _, metric := range scope.Metrics {
			if metric.Name == name {
				return metric
			}
		}
	}
	t.Fatalf("metric %q was not collected", name)
	return metricdata.Metrics{}
}

func assertMetricRoute(t *testing.T, attributes attribute.Set) {
	t.Helper()
	value, ok := attributes.Value(attribute.Key("route"))
	if !ok || value.AsString() != "/users" {
		t.Fatalf("route attribute = %v, %v; want /users, true", value, ok)
	}
}
