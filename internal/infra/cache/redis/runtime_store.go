package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"go-backend-architecture/internal/repository"
)

type RuntimeStore struct {
	client   *goredis.Client
	cacheTTL time.Duration
}

func NewRuntimeStore(client *goredis.Client, cacheTTL time.Duration) *RuntimeStore {
	return &RuntimeStore{
		client:   client,
		cacheTTL: cacheTTL,
	}
}

func (c *RuntimeStore) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

func (c *RuntimeStore) GetSearchRuntimeValues(ctx context.Context, prefix string, limit uint64) ([]repository.RuntimeKV, bool, error) {
	key := searchRuntimeValuesKey(prefix, limit)
	raw, err := c.client.Get(ctx, key).Result()
	if err == goredis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("redis get key %q: %w", key, err)
	}

	var items []repository.RuntimeKV
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil, false, fmt.Errorf("unmarshal key %q payload: %w", key, err)
	}
	return items, true, nil
}

func (c *RuntimeStore) SetSearchRuntimeValues(ctx context.Context, prefix string, limit uint64, items []repository.RuntimeKV) error {
	key := searchRuntimeValuesKey(prefix, limit)
	payload, err := json.Marshal(items)
	if err != nil {
		return fmt.Errorf("marshal key %q payload: %w", key, err)
	}
	if err := c.client.Set(ctx, key, payload, c.cacheTTL).Err(); err != nil {
		return fmt.Errorf("redis set key %q: %w", key, err)
	}
	return nil
}

func searchRuntimeValuesKey(prefix string, limit uint64) string {
	return "runtime:search:prefix=" + url.QueryEscape(prefix) + ":limit=" + strconv.FormatUint(limit, 10)
}
