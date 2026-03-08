package runtimerepo

import (
	"context"

	"go-backend-architecture/internal/infra/logger"
	"go-backend-architecture/internal/infra/observability"
	"go-backend-architecture/internal/repository"
)

type RuntimeCachedRepository struct {
	base   repository.RuntimeRepository
	cache  runtimeCacheStore
	log    logger.Logger
	tracer observability.Tracer
}

type runtimeCacheStore interface {
	GetSearchRuntimeValues(ctx context.Context, prefix string, limit uint64) (items []repository.RuntimeKV, found bool, err error)
	SetSearchRuntimeValues(ctx context.Context, prefix string, limit uint64, items []repository.RuntimeKV) error
}

func NewRuntimeCachedRepository(log logger.Logger, tracer observability.Tracer, base repository.RuntimeRepository, cache runtimeCacheStore) *RuntimeCachedRepository {
	return &RuntimeCachedRepository{
		base:   base,
		cache:  cache,
		log:    log,
		tracer: tracer,
	}
}

func (r *RuntimeCachedRepository) Ping(ctx context.Context) error {
	return r.base.Ping(ctx)
}

func (r *RuntimeCachedRepository) GetServerStatus(ctx context.Context) (repository.DBServerStatus, error) {
	return r.base.GetServerStatus(ctx)
}

func (r *RuntimeCachedRepository) GetRuntimeValue(ctx context.Context, key string) (repository.RuntimeKV, error) {
	return r.base.GetRuntimeValue(ctx, key)
}

func (r *RuntimeCachedRepository) SearchRuntimeValues(ctx context.Context, prefix string, limit uint64) (items []repository.RuntimeKV, err error) {
	ctx, span := r.tracer.Start(ctx, "repository", "runtime_cached_repository.search_runtime_values",
		observability.FromPairs(
			"cache.system", "redis",
			"cache.operation", "cache_aside",
			"query.prefix", prefix,
			"query.limit", int64(limit),
		),
	)
	defer func() {
		span.Finish(err)
	}()

	cachedItems, found, cacheErr := r.cache.GetSearchRuntimeValues(ctx, prefix, limit)
	if cacheErr != nil {
		span.SetAttributes(observability.FromPairs("cache.read.error", true))
		r.log.Warn(ctx, "redis cache read failed, fallback to database",
			logger.FromPairs(
				"prefix", prefix,
				"limit", limit,
				"error", cacheErr.Error(),
			),
		)
	} else if found {
		span.SetAttributes(observability.FromPairs("cache.hit", true))
		return cachedItems, nil
	}

	span.SetAttributes(observability.FromPairs("cache.hit", false))
	items, err = r.base.SearchRuntimeValues(ctx, prefix, limit)
	if err != nil {
		return nil, err
	}

	if cacheErr := r.cache.SetSearchRuntimeValues(ctx, prefix, limit, items); cacheErr != nil {
		span.SetAttributes(observability.FromPairs("cache.write.error", true))
		r.log.Warn(ctx, "redis cache write failed",
			logger.FromPairs(
				"prefix", prefix,
				"limit", limit,
				"error", cacheErr.Error(),
			),
		)
	}

	return items, nil
}
