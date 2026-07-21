package ratelimitmw

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/usecase/globalratelimit"
	"github.com/labstack/echo/v5"
)

type limiterStub struct {
	d   globalratelimit.Decision
	err error
}

func (s limiterStub) AllowIP(context.Context, string) (globalratelimit.Decision, error) {
	return s.d, s.err
}
func TestGlobalRateLimitMiddlewareRejectsWithRetryAfter(t *testing.T) {
	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), rec)
	h := NewGlobalRateLimit(limiterStub{d: globalratelimit.Decision{RetryAfter: 500 * time.Millisecond}}, nil, nil).Handler()(func(*echo.Context) error { t.Fatal("next called"); return nil })
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
