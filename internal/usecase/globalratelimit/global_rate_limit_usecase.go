package globalratelimit

import (
	"context"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	"github.com/eannchen/go-backend-architecture/internal/logger"
	repokvstore "github.com/eannchen/go-backend-architecture/internal/repository/kvstore"
)

type Decision struct {
	Allowed    bool
	RetryAfter time.Duration
}

type Limiter interface {
	AllowIP(context.Context, string) (Decision, error)
}

type Config struct {
	Capacity       int
	RefillInterval time.Duration
}

type ipLimiter struct {
	repo repokvstore.TokenBucketRepository
	log  logger.Logger
	cfg  Config
}

func NewIPLimiter(repo repokvstore.TokenBucketRepository, log logger.Logger, cfg Config) Limiter {
	if log == nil {
		log = logger.NoopLogger{}
	}
	return &ipLimiter{repo, log, cfg}
}

func (l *ipLimiter) AllowIP(ctx context.Context, ip string) (Decision, error) {
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
