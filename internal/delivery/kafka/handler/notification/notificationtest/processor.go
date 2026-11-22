package notificationtest

import (
	"context"
	"sync"

	"github.com/eannchen/go-backend-architecture/internal/usecase/notification"
)

// Processor is a configurable double for its parent package's Processor contract.
// Configure callbacks before use and inspect calls after concurrent work finishes.
type Processor struct {
	mu          sync.Mutex
	HandleFunc  func(ctx context.Context, request notification.Request) error
	HandleCalls []ProcessorHandleCall
}

// ProcessorHandleCall records one Processor.Handle invocation.
type ProcessorHandleCall struct {
	Context context.Context
	Request notification.Request
}

func (d *Processor) Handle(ctx context.Context, request notification.Request) error {
	d.mu.Lock()
	d.HandleCalls = append(d.HandleCalls, ProcessorHandleCall{Context: ctx, Request: request})
	fn := d.HandleFunc
	d.mu.Unlock()
	if fn == nil {
		panic("unexpected Processor.Handle call")
	}
	return fn(ctx, request)
}
