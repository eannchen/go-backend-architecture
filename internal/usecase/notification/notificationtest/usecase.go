package notificationtest

import (
	"context"
	"sync"

	"github.com/eannchen/go-backend-architecture/internal/usecase/notification"
)

// Usecase is the canonical configurable double for notification.Usecase.
type Usecase struct {
	mu          sync.Mutex
	HandleFunc  func(context.Context, notification.Request) error
	HandleCalls []notification.Request
}

func (u *Usecase) Handle(ctx context.Context, request notification.Request) error {
	u.mu.Lock()
	u.HandleCalls = append(u.HandleCalls, request)
	u.mu.Unlock()
	if u.HandleFunc == nil {
		panic("unexpected notification Usecase.Handle call")
	}
	return u.HandleFunc(ctx, request)
}
