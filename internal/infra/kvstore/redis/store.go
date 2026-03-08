package redis

import (
	"context"
	"fmt"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"go-backend-architecture/internal/repository"
)

type Store struct {
	client *goredis.Client
	prefix string
}

func NewStore(client *goredis.Client, prefix string) *Store {
	return &Store{
		client: client,
		prefix: strings.TrimSpace(prefix),
	}
}

func (s *Store) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	if key == "" {
		return fmt.Errorf("kv key must not be empty")
	}
	if err := s.client.Set(ctx, s.prefixedKey(key), value, ttl).Err(); err != nil {
		return fmt.Errorf("kv set key %q: %w", key, err)
	}
	return nil
}

func (s *Store) Get(ctx context.Context, key string) (value string, found bool, err error) {
	if key == "" {
		return "", false, fmt.Errorf("kv key must not be empty")
	}
	value, err = s.client.Get(ctx, s.prefixedKey(key)).Result()
	if err == goredis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("kv get key %q: %w", key, err)
	}
	return value, true, nil
}

func (s *Store) Delete(ctx context.Context, key string) error {
	if key == "" {
		return fmt.Errorf("kv key must not be empty")
	}
	if err := s.client.Del(ctx, s.prefixedKey(key)).Err(); err != nil {
		return fmt.Errorf("kv delete key %q: %w", key, err)
	}
	return nil
}

func (s *Store) prefixedKey(key string) string {
	if s.prefix == "" {
		return key
	}
	return s.prefix + ":" + key
}

var _ repository.KVStore = (*Store)(nil)
