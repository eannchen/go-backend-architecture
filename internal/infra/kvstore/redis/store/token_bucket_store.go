package store

import (
	"context"
	"fmt"
	"time"

	repokvstore "github.com/eannchen/go-backend-architecture/internal/repository/kvstore"
	goredis "github.com/redis/go-redis/v9"
)

const tokenBucketKeyPrefix = "ratelimit:tb:"

var tokenBucketScript = goredis.NewScript(`
local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local refill_ms = tonumber(ARGV[2])
local now_ms = tonumber(ARGV[3])

local data = redis.call('HMGET', key, 't', 'u')
local tokens = tonumber(data[1])
local updated = tonumber(data[2])

if tokens == nil then
    tokens = capacity
    updated = now_ms
end

local elapsed = now_ms - updated
if elapsed < 0 then elapsed = 0 end

tokens = math.min(capacity, tokens + (elapsed / refill_ms))

local allowed = 0
local retry_ms = 0
if tokens >= 1 then
    tokens = tokens - 1
    allowed = 1
else
    retry_ms = math.ceil((1 - tokens) * refill_ms)
end

redis.call('HSET', key, 't', tostring(tokens), 'u', now_ms)
redis.call('PEXPIRE', key, math.ceil(capacity * refill_ms))

return {allowed, retry_ms, math.floor(tokens)}`)

type TokenBucketStore struct{ client *goredis.Client }

func NewTokenBucketStore(client *goredis.Client) *TokenBucketStore {
	return &TokenBucketStore{client: client}
}

func (s *TokenBucketStore) Allow(ctx context.Context, key string, capacity int, refill time.Duration) (repokvstore.TokenBucketDecision, error) {
	ms := refill.Milliseconds()
	if capacity <= 0 || ms <= 0 {
		return repokvstore.TokenBucketDecision{}, fmt.Errorf("token bucket: invalid capacity or refill interval")
	}
	res, err := tokenBucketScript.Run(ctx, s.client, []string{tokenBucketKeyPrefix + key}, capacity, ms, time.Now().UnixMilli()).Slice()
	if err != nil {
		return repokvstore.TokenBucketDecision{}, fmt.Errorf("token bucket allow: %w", err)
	}
	allowed, err := redisInt(res[0])
	if err != nil {
		return repokvstore.TokenBucketDecision{}, err
	}
	retry, err := redisInt(res[1])
	if err != nil {
		return repokvstore.TokenBucketDecision{}, err
	}
	remaining, err := redisInt(res[2])
	if err != nil {
		return repokvstore.TokenBucketDecision{}, err
	}
	d := repokvstore.TokenBucketDecision{Allowed: allowed == 1, Remaining: remaining}
	if !d.Allowed && retry > 0 {
		d.RetryAfter = time.Duration(retry) * time.Millisecond
	}
	return d, nil
}

var _ repokvstore.TokenBucketRepository = (*TokenBucketStore)(nil)
