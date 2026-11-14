package recovery

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware"
	"github.com/eannchen/go-backend-architecture/internal/logger"
)

// Middleware logs handler panics and converts them into safe processing failures.
type Middleware struct{ log logger.Logger }

func New(log logger.Logger) *Middleware {
	return &Middleware{log: log}
}

func (m *Middleware) Handler() middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, record *kgo.Record) (err error) {
			defer func() {
				if recover() != nil {
					// Panic values can contain private payloads. Record a safe error and the
					// stack location. Stack frames exclude argument values as well.
					err = middleware.ErrHandlerPanic
					m.log.Error(ctx, "Kafka panic recovered", err, logger.FromPairs(
						"messaging.system", "kafka", "messaging.destination.name", record.Topic,
						"messaging.destination.partition.id", record.Partition,
						"messaging.kafka.offset", record.Offset, "panic.stack", panicStack()))
				}
			}()
			return next(ctx, record)
		}
	}
}

// panicStack includes function names and source locations without argument values.
func panicStack() string {
	var pcs [64]uintptr
	count := runtime.Callers(3, pcs[:])
	frames := runtime.CallersFrames(pcs[:count])
	var stack strings.Builder
	for {
		frame, more := frames.Next()
		fmt.Fprintf(&stack, "%s\n\t%s:%d\n", frame.Function, frame.File, frame.Line)
		if !more {
			break
		}
	}
	return stack.String()
}
