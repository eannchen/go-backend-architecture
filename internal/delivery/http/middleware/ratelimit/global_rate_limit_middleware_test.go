package ratelimitmw

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	"github.com/eannchen/go-backend-architecture/internal/observability/observabilitytest"
	"github.com/eannchen/go-backend-architecture/internal/usecase/globalratelimit"
	"github.com/eannchen/go-backend-architecture/internal/usecase/globalratelimit/globalratelimittest"
)

func TestGlobalRateLimitMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		decision       globalratelimit.Decision
		limiterErr     error
		wantStatus     int
		wantNextCalls  int
		wantRetryAfter string
		wantOutcome    string
	}{
		{
			name:          "allows request",
			decision:      globalratelimit.Decision{Allowed: true},
			wantStatus:    http.StatusNoContent,
			wantNextCalls: 1,
			wantOutcome:   "allowed",
		},
		{
			name:           "rejects request and rounds retry upward",
			decision:       globalratelimit.Decision{RetryAfter: 1100 * time.Millisecond},
			wantStatus:     http.StatusTooManyRequests,
			wantRetryAfter: "2",
			wantOutcome:    "rate_limited",
		},
		{
			name:        "reports limiter failure",
			limiterErr:  apperr.New(apperr.CodeUnavailable, "rate limiter unavailable"),
			wantStatus:  http.StatusServiceUnavailable,
			wantOutcome: "backend_unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter := &globalratelimittest.Limiter{
				AllowIPFunc: func(context.Context, string) (globalratelimit.Decision, error) {
					return tt.decision, tt.limiterErr
				},
			}
			meter := observabilitytest.NewRecordingMeter()
			middleware := NewGlobalRateLimit(limiter, nil, meter)
			e := echo.New()
			rec := httptest.NewRecorder()
			c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), rec)
			nextCalls := 0

			err := middleware.Handler()(func(c *echo.Context) error {
				nextCalls++
				return c.NoContent(http.StatusNoContent)
			})(c)

			if err != nil {
				t.Fatalf("middleware error = %v", err)
			}
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if nextCalls != tt.wantNextCalls {
				t.Fatalf("next calls = %d, want %d", nextCalls, tt.wantNextCalls)
			}
			if got := rec.Header().Get("Retry-After"); got != tt.wantRetryAfter {
				t.Fatalf("Retry-After = %q, want %q", got, tt.wantRetryAfter)
			}
			if limiter.AllowIPCalls != 1 || limiter.AllowIPIP == "" {
				t.Fatalf("limiter call = %d, %q; want one non-empty IP", limiter.AllowIPCalls, limiter.AllowIPIP)
			}
			samples := meter.CounterSamples("http_rate_limit_decisions_total")
			if len(samples) != 1 || samples[0].Fields["outcome"] != tt.wantOutcome {
				t.Fatalf("decision metrics = %#v, want outcome %q", samples, tt.wantOutcome)
			}
		})
	}
}

func TestRetryAfterSeconds(t *testing.T) {
	tests := []struct {
		duration time.Duration
		want     int
	}{
		{duration: 0, want: 1},
		{duration: time.Millisecond, want: 1},
		{duration: time.Second, want: 1},
		{duration: time.Second + time.Millisecond, want: 2},
	}

	for _, tt := range tests {
		if got := retryAfterSeconds(tt.duration); got != tt.want {
			t.Fatalf("retryAfterSeconds(%v) = %d, want %d", tt.duration, got, tt.want)
		}
	}
}
