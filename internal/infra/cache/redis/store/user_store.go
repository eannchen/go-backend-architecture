package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	goredis "github.com/redis/go-redis/v9"

	domainuser "github.com/eannchen/go-backend-architecture/internal/domain/user"
	repocache "github.com/eannchen/go-backend-architecture/internal/repository/cache"
)

const userKeyPrefix = "user:id:"

// UserCacheStore caches user data in Redis with TTL.
type UserCacheStore struct {
	client   *goredis.Client
	cacheTTL time.Duration
}

func NewUserCacheStore(client *goredis.Client, cacheTTL time.Duration) *UserCacheStore {
	return &UserCacheStore{client: client, cacheTTL: cacheTTL}
}

func (s *UserCacheStore) GetByID(ctx context.Context, id int64) (domainuser.User, bool, error) {
	key := userKeyPrefix + strconv.FormatInt(id, 10)
	data, err := s.client.Get(ctx, key).Bytes()
	if err == goredis.Nil {
		return domainuser.User{}, false, nil
	}
	if err != nil {
		return domainuser.User{}, false, fmt.Errorf("redis get key %q: %w", key, err)
	}
	var user domainuser.User
	if err := json.Unmarshal(data, &user); err != nil {
		return domainuser.User{}, false, fmt.Errorf("unmarshal key %q payload: %w", key, err)
	}
	return user, true, nil
}

func (s *UserCacheStore) SetByID(ctx context.Context, id int64, user domainuser.User) error {
	key := userKeyPrefix + strconv.FormatInt(id, 10)
	data, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("marshal user for key %q: %w", key, err)
	}
	if err := s.client.Set(ctx, key, data, s.cacheTTL).Err(); err != nil {
		return fmt.Errorf("redis set key %q: %w", key, err)
	}
	return nil
}

func (s *UserCacheStore) DeleteByID(ctx context.Context, id int64) error {
	key := userKeyPrefix + strconv.FormatInt(id, 10)
	if err := s.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("redis del key %q: %w", key, err)
	}
	return nil
}

var _ repocache.UserCacheStore = (*UserCacheStore)(nil)
