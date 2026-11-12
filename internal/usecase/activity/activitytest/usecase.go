package activitytest

import (
	"context"
	"sync"

	"github.com/eannchen/go-backend-architecture/internal/usecase/activity"
)

// Usecase is the canonical configurable double for activity.Usecase.
type Usecase struct {
	mu          sync.Mutex
	HandleFunc  func(context.Context, activity.Request) error
	HandleCalls []activity.Request
}

func (u *Usecase) Handle(ctx context.Context, request activity.Request) error {
	u.mu.Lock()
	u.HandleCalls = append(u.HandleCalls, request)
	u.mu.Unlock()
	if u.HandleFunc == nil {
		panic("unexpected activity Usecase.Handle call")
	}
	return u.HandleFunc(ctx, request)
}
