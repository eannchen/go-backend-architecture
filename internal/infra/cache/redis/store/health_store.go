package store

import (
	"context"

	goredis "github.com/redis/go-redis/v9"

	"go-backend-architecture/internal/repository"
)

type HealthStore struct {
	client *goredis.Client
}

func NewHealthStore(client *goredis.Client) *HealthStore {
	return &HealthStore{client: client}
}

func (s *HealthStore) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

var _ repository.CacheHealthStore = (*HealthStore)(nil)
