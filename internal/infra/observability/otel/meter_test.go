package otel

import (
	"sync"
	"testing"

	"go.opentelemetry.io/otel/metric/noop"
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
