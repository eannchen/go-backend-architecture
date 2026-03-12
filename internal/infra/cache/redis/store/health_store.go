package store

import (
	"context"
	"fmt"

	goredis "github.com/redis/go-redis/v9"

	"github.com/eannchen/go-backend-architecture/internal/repository"
)

type HealthStore struct {
	client *goredis.Client
}

func NewHealthStore(client *goredis.Client) *HealthStore {
	return &HealthStore{client: client}
}

func (s *HealthStore) Ping(ctx context.Context) error {
	err := s.client.Ping(ctx).Err()
	if err != nil {
		return fmt.Errorf("redis ping failed: %w", err)
	}
	return nil
}

var _ repository.CacheHealthStore = (*HealthStore)(nil)
