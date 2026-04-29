package user

import (
	"context"

	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/observability"
	repocache "github.com/eannchen/go-backend-architecture/internal/repository/cache"
	repodb "github.com/eannchen/go-backend-architecture/internal/repository/db"
)

// CachedUserStore wraps a DB-backed UserRepository with a cache layer (cache-aside pattern).
// Reads check cache first; writes pass through to DB and invalidate the cache entry.
type CachedUserStore struct {
	base   repodb.UserRepository
	cache  repocache.UserCacheStore
	log    logger.Logger
	tracer observability.Tracer
}

func NewCachedUserStore(
	log logger.Logger,
	tracer observability.Tracer,
	base repodb.UserRepository,
	cache repocache.UserCacheStore,
) *CachedUserStore {
	if log == nil {
		log = logger.NoopLogger{}
	}
	if tracer == nil {
		tracer = observability.NoopTracer{}
	}
	return &CachedUserStore{base: base, cache: cache, log: log, tracer: tracer}
}

func (r *CachedUserStore) GetByID(ctx context.Context, id int64) (user repodb.User, err error) {
	ctx, span := r.tracer.Start(ctx, "repository", "cached_user_store.get_by_id",
		observability.FromPairs("user.id", id),
	)
	defer func() { span.Finish(err) }()

	cached, found, cacheErr := r.cache.GetByID(ctx, id)
	if cacheErr != nil {
		r.log.Warn(ctx, "user cache read failed", logger.FromPairs("user.id", id, "error", cacheErr))
	}
	if found {
		span.SetAttributes(observability.FromPairs("cache", "hit"))
		return cached, nil
	}
	span.SetAttributes(observability.FromPairs("cache", "miss"))

	user, err = r.base.GetByID(ctx, id)
	if err != nil {
		return repodb.User{}, err
	}

	if cacheErr := r.cache.SetByID(ctx, id, user); cacheErr != nil {
		r.log.Warn(ctx, "user cache write failed", logger.FromPairs("user.id", id, "error", cacheErr))
	}
	return user, nil
}

// GetByEmail bypasses cache — email-based lookups are infrequent (login only).
func (r *CachedUserStore) GetByEmail(ctx context.Context, email string) (repodb.User, error) {
	return r.base.GetByEmail(ctx, email)
}

func (r *CachedUserStore) CreateByEmail(ctx context.Context, email string) (repodb.User, error) {
	return r.base.CreateByEmail(ctx, email)
}

func (r *CachedUserStore) UpsertOAuthUser(ctx context.Context, info repodb.OAuthUserUpsert) (repodb.User, error) {
	user, err := r.base.UpsertOAuthUser(ctx, info)
	if err != nil {
		return repodb.User{}, err
	}
	if cacheErr := r.cache.DeleteByID(ctx, user.ID); cacheErr != nil {
		r.log.Warn(ctx, "user cache invalidation failed", logger.FromPairs("user.id", user.ID, "error", cacheErr))
	}
	return user, nil
}

var _ repodb.UserRepository = (*CachedUserStore)(nil)
