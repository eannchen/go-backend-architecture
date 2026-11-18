package store

import (
	"context"
	"fmt"

	goredis "github.com/redis/go-redis/v9"
)

type HealthStore struct {
	client *goredis.Client
}

func NewHealthStore(client *goredis.Client) *HealthStore {
	return &HealthStore{client: client}
}

func (s *HealthStore) Ping(ctx context.Context) error {
	if err := s.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis kv ping failed: %w", err)
	}
	return nil
}
