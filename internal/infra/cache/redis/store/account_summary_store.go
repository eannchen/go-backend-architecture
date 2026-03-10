package store

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/eannchen/go-backend-architecture/internal/repository"
)

type AccountSummaryStore struct {
	client   *goredis.Client
	cacheTTL time.Duration
}

func NewAccountSummaryStore(client *goredis.Client, cacheTTL time.Duration) *AccountSummaryStore {
	return &AccountSummaryStore{
		client:   client,
		cacheTTL: cacheTTL,
	}
}

func (c *AccountSummaryStore) GetAccountSummaryByID(ctx context.Context, id int64) (repository.AccountSummary, bool, error) {
	key := accountSummaryByIDKey(id)
	raw, err := c.client.Get(ctx, key).Result()
	if err == goredis.Nil {
		return repository.AccountSummary{}, false, nil
	}
	if err != nil {
		return repository.AccountSummary{}, false, fmt.Errorf("redis get key %q: %w", key, err)
	}

	var item repository.AccountSummary
	if err := json.Unmarshal([]byte(raw), &item); err != nil {
		return repository.AccountSummary{}, false, fmt.Errorf("unmarshal key %q payload: %w", key, err)
	}
	return item, true, nil
}

func (c *AccountSummaryStore) SetAccountSummaryByID(ctx context.Context, id int64, item repository.AccountSummary) error {
	key := accountSummaryByIDKey(id)
	payload, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal key %q payload: %w", key, err)
	}
	if err := c.client.Set(ctx, key, payload, c.cacheTTL).Err(); err != nil {
		return fmt.Errorf("redis set key %q: %w", key, err)
	}
	return nil
}

func accountSummaryByIDKey(id int64) string {
	return "account_summary:id=" + url.QueryEscape(strconv.FormatInt(id, 10))
}
