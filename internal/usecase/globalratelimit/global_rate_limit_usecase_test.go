package globalratelimit_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/usecase/globalratelimit"
	globalratelimittest "github.com/eannchen/go-backend-architecture/internal/usecase/globalratelimit/globalratelimittest"
)

// TestAllowIP checks IP rate-limit decisions and dependency failures map to usecase outcomes.
func TestAllowIP(t *testing.T) {
	tests := []struct {
		name, ip string
		decision globalratelimit.StoreDecision
		repoErr  error
		code     apperr.Code
		allowed  bool
	}{
		{"allows", "1.2.3.4", globalratelimit.StoreDecision{Allowed: true}, nil, "", true},
		{"denies", "1.2.3.4", globalratelimit.StoreDecision{}, nil, "", false},
		{"missing IP", "", globalratelimit.StoreDecision{}, nil, apperr.CodeTooManyRequests, false},
		{"store error", "1.2.3.4", globalratelimit.StoreDecision{}, errors.New("redis down"), apperr.CodeUnavailable, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &globalratelimittest.TokenBucketRepository{AllowFunc: func(context.Context, string, int, time.Duration) (globalratelimit.StoreDecision, error) {
				return tt.decision, tt.repoErr
			}}
			d, err := globalratelimit.NewIPLimiter(repo, logger.NoopLogger{}, globalratelimit.Config{10, time.Second}).AllowIP(context.Background(), tt.ip)
			if tt.code != "" {
				a, ok := apperr.As(err)
				if !ok || a.Code != tt.code {
					t.Fatalf("error=%v", err)
				}
			} else if err != nil || d.Allowed != tt.allowed {
				t.Fatalf("decision=%+v error=%v", d, err)
			}
		})
	}
}
