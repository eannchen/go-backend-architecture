// Package kafkaprocessing assembles the processing policy shared by Kafka apps.
package kafkaprocessing

import (
	"time"

	contextmw "github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware/context"

	observabilitymw "github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware/observability"
	recoverymw "github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware/recovery"
	"github.com/eannchen/go-backend-architecture/internal/logger"

	"github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware"
	"github.com/eannchen/go-backend-architecture/internal/observability"
)

// Config comes from validated app configuration. HandlingTimeout must be positive
// and AcknowledgmentMargin nonnegative. Traditional consumers use a zero margin.
type Config struct {
	HandlingTimeout      time.Duration
	AcknowledgmentMargin time.Duration
}

// Wrapper is constructed once during app wiring and reused for its handlers.
// Different workers can use separate wrappers when their time budgets differ.
type Wrapper struct {
	observability *observabilitymw.Middleware
	recovery      middleware.Middleware
	context       middleware.Middleware
}

func New(tracer observability.Tracer, log logger.Logger, meter observability.Meter, cfg Config) *Wrapper {
	return &Wrapper{
		observability: observabilitymw.New(tracer, log, meter),
		recovery:      recoverymw.New(log).Handler(),
		context:       contextmw.New(tracer, contextmw.Config{HandlingTimeout: cfg.HandlingTimeout, AcknowledgmentMargin: cfg.AcknowledgmentMargin}).Handler(),
	}
}

// Wrap applies the app's policy to one handler. operation is a fixed name chosen
// in app wiring, so each handler has its own trace name and metric label.
func (w *Wrapper) Wrap(operation string, handler middleware.Handler) middleware.Handler {
	// Listed outermost first: observability sees recovered panics and deadlines.
	return middleware.Chain(handler, w.context,
		w.observability.Handler(operation),
		w.recovery)
}
