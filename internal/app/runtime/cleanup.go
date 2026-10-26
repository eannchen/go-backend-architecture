package runtime

import (
	"context"
	"time"
)

// CleanupAfterInitFailure gives partially initialized resources a fresh cleanup
// budget even when startup was canceled or its deadline expired. Context values
// are preserved for logging/tracing. Shutdown must honor the cleanup context;
// this helper cannot interrupt a blocking operation that ignores cancellation.
func CleanupAfterInitFailure(ctx context.Context, timeout time.Duration, shutdown func(context.Context) error) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()
	return shutdown(cleanupCtx)
}
