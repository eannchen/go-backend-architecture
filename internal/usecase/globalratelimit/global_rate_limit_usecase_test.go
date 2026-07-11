package globalratelimit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	"github.com/eannchen/go-backend-architecture/internal/logger"
	repokvstore "github.com/eannchen/go-backend-architecture/internal/repository/kvstore"
	"github.com/eannchen/go-backend-architecture/internal/repository/kvstore/kvstoretest"
)

func TestAllowIP(t *testing.T) {
	tests := []struct {
		name, ip string
		decision repokvstore.TokenBucketDecision
		repoErr  error
		code     apperr.Code
		allowed  bool
	}{
		{"allows", "1.2.3.4", repokvstore.TokenBucketDecision{Allowed: true}, nil, "", true},
		{"denies", "1.2.3.4", repokvstore.TokenBucketDecision{}, nil, "", false},
		{"missing IP", "", repokvstore.TokenBucketDecision{}, nil, apperr.CodeTooManyRequests, false},
		{"store error", "1.2.3.4", repokvstore.TokenBucketDecision{}, errors.New("redis down"), apperr.CodeUnavailable, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &kvstoretest.TokenBucketRepository{AllowFunc: func(context.Context, string, int, time.Duration) (repokvstore.TokenBucketDecision, error) {
				return tt.decision, tt.repoErr
			}}
			d, err := NewIPLimiter(repo, logger.NoopLogger{}, Config{10, time.Second}).AllowIP(context.Background(), tt.ip)
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
