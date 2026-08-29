package ratelimitmw

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/usecase/globalratelimit"
	"github.com/eannchen/go-backend-architecture/internal/usecase/globalratelimit/globalratelimittest"
	"github.com/labstack/echo/v5"
)

func TestGlobalRateLimitMiddlewareRejectsWithRetryAfter(t *testing.T) {
	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), rec)
	limiter := &globalratelimittest.Limiter{
		AllowIPFunc: func(_ context.Context, _ string) (globalratelimit.Decision, error) {
			return globalratelimit.Decision{RetryAfter: 500 * time.Millisecond}, nil
		},
	}
	h := NewGlobalRateLimit(limiter, nil, nil).Handler()(func(*echo.Context) error { t.Fatal("next called"); return nil })
	if err := h(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status=%d", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "1" {
		t.Fatalf("retry=%q", got)
	}
}
