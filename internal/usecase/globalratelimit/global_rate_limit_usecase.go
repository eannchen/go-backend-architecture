package globalratelimit

import (
	"context"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	"github.com/eannchen/go-backend-architecture/internal/logger"
)

type Decision struct {
	Allowed    bool
	RetryAfter time.Duration
}

type Config struct {
	Capacity       int
	RefillInterval time.Duration
}

// IPLimiter implements the global IP rate-limit policy.
type IPLimiter struct {
	repo TokenBucketRepository
	log  logger.Logger
	cfg  Config
}

func NewIPLimiter(repo TokenBucketRepository, log logger.Logger, cfg Config) *IPLimiter {
	return &IPLimiter{repo, log, cfg}
}

func (l *IPLimiter) AllowIP(ctx context.Context, ip string) (Decision, error) {
	if ip == "" {
		return Decision{}, apperr.New(apperr.CodeTooManyRequests, "rate limit: client IP unavailable")
	}
	d, err := l.repo.Allow(ctx, "global:ip:"+ip, l.cfg.Capacity, l.cfg.RefillInterval)
	if err != nil {
		l.log.Warn(ctx, "global rate limit lookup failed", logger.FromPairs("ip", ip, "error", err))
		return Decision{}, apperr.Wrap(err, apperr.CodeUnavailable, "rate limiter unavailable")
	}
	return Decision{Allowed: d.Allowed, RetryAfter: d.RetryAfter}, nil
}
