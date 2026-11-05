package otel

import (
	"errors"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/plugin/kotel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// KafkaHooks builds fresh client hooks from explicitly supplied providers.
// Our Kafka constructors attach these hooks with kgo.WithHooks; franz-go then
// calls them as records enter and leave its buffers.
//
// Publishing:
//   - Start a publish span using record.Context as its parent context.
//   - Write its trace identity into the Kafka headers and save the span in
//     record.Context. End the span when publishing completes, recording failures.
//
// Receiving:
//   - Read the trace identity from the Kafka headers and start a receive span.
//     Valid headers continue the producer's trace with a new span ID. Without
//     headers or another valid parent context, the SDK starts a new trace.
//   - Save that span in record.Context. End it when the record leaves the client
//     buffer, before our worker finishes processing it.
//
// Headers travel through Kafka; record.Context exists only in this Go process.
// Our delivery tracing helper carries the receive span's identity onto the worker
// context, preserving shutdown cancellation, then the worker starts a separate
// processing span. An ended receive span can still be that span's parent.
//
// Close the Kafka client before shutting down the runtime that owns the providers.
func KafkaHooks(tracerProvider trace.TracerProvider, meterProvider metric.MeterProvider, group string) []kgo.Hook {
	tracer := kotel.NewTracer(
		kotel.TracerProvider(tracerProvider),
		// Use W3C traceparent/tracestate headers to connect producer and consumer
		// traces. This propagator does not copy application baggage into messages.
		kotel.TracerPropagator(propagation.TraceContext{}),
		kotel.ConsumerGroup(group),
		// Keys may contain recipient identifiers. Returning an error omits the
		// attribute entirely. Record keys and Kafka partition routing stay intact.
		kotel.KeyFormatter(func(*kgo.Record) (string, error) { return "", errors.New("message keys are private") }),
	)
	// These metrics describe Kafka client activity, such as network traffic and
	// record batches. They do not measure our usecase's processing time or success.
	meter := kotel.NewMeter(kotel.MeterProvider(meterProvider))
	return kotel.NewKotel(kotel.WithTracer(tracer), kotel.WithMeter(meter)).Hooks()
}
