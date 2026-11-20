package runtime

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// CleanupAfterInitFailure gives partially initialized resources a fresh cleanup
// budget even when startup was canceled or its deadline expired. Context values
// are preserved for logging/tracing. Shutdown must honor the cleanup context.
// Functions run in the supplied order and share one budget. All are attempted,
// even after a failure. Callers supply reverse dependency order.
// This helper cannot interrupt a blocking operation that ignores cancellation.
func CleanupAfterInitFailure(ctx context.Context, timeout time.Duration, shutdowns ...func(context.Context) error) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()
	var result error
	for _, shutdown := range shutdowns {
		result = errors.Join(result, shutdown(cleanupCtx))
	}
	return result
}

// WrapCleanupError adds operation context to a cleanup failure and preserves its
// cause. Successful cleanup stays nil so independent results can be joined.
func WrapCleanupError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", operation, err)
}
