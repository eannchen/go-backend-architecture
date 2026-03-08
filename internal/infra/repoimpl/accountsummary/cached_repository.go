package accountsummary

import (
	"context"

	"go-backend-architecture/internal/infra/logger"
	"go-backend-architecture/internal/infra/observability"
	"go-backend-architecture/internal/repository"
)

type CachedRepository struct {
	base   repository.AccountSummaryRepository
	cache  accountSummaryCacheStore
	log    logger.Logger
	tracer observability.Tracer
}

type accountSummaryCacheStore interface {
	GetAccountSummaryByID(ctx context.Context, id int64) (item repository.AccountSummary, found bool, err error)
	SetAccountSummaryByID(ctx context.Context, id int64, item repository.AccountSummary) error
}

func NewCachedRepository(log logger.Logger, tracer observability.Tracer, base repository.AccountSummaryRepository, cache accountSummaryCacheStore) *CachedRepository {
	return &CachedRepository{
		base:   base,
		cache:  cache,
		log:    log,
		tracer: tracer,
	}
}

func (r *CachedRepository) GetByID(ctx context.Context, id int64) (item repository.AccountSummary, err error) {
	ctx, span := r.tracer.Start(ctx, "repository", "account_summary_cached_repository.get_by_id",
		observability.FromPairs(
			"cache.system", "redis",
			"cache.operation", "cache_aside",
			"account.id", id,
		),
	)
	defer func() {
		span.Finish(err)
	}()

	cachedItem, found, cacheErr := r.cache.GetAccountSummaryByID(ctx, id)
	if cacheErr != nil {
		span.SetAttributes(observability.FromPairs("cache.read.error", true))
		r.log.Warn(ctx, "redis cache read failed, fallback to database",
			logger.FromPairs(
				"account.id", id,
				"error", cacheErr.Error(),
			),
		)
	} else if found {
		span.SetAttributes(observability.FromPairs("cache.hit", true))
		return cachedItem, nil
	}

	span.SetAttributes(observability.FromPairs("cache.hit", false))
	item, err = r.base.GetByID(ctx, id)
	if err != nil {
		return repository.AccountSummary{}, err
	}

	if cacheErr := r.cache.SetAccountSummaryByID(ctx, id, item); cacheErr != nil {
		span.SetAttributes(observability.FromPairs("cache.write.error", true))
		r.log.Warn(ctx, "redis cache write failed",
			logger.FromPairs(
				"account.id", id,
				"error", cacheErr.Error(),
			),
		)
	}

	return item, nil
}
