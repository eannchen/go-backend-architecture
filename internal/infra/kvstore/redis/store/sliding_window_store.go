package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/eannchen/go-backend-architecture/internal/usecase/globalratelimit"
)

const slidingWindowKeyPrefix = "ratelimit:sw:"

var slidingWindowScript = goredis.NewScript(`
local key = KEYS[1]
local now_ms = tonumber(ARGV[1])
local window_ms = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local member = ARGV[4]

redis.call('ZREMRANGEBYSCORE', key, '-inf', now_ms - window_ms)

local count = redis.call('ZCARD', key)
if count >= limit then
    local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
    local retry_ms = window_ms
    if oldest[2] ~= nil then
        retry_ms = tonumber(oldest[2]) + window_ms - now_ms
    end
    if retry_ms < 1 then retry_ms = 1 end
    return {0, retry_ms, 0}
end

redis.call('ZADD', key, now_ms, member)
redis.call('PEXPIRE', key, window_ms)
return {1, 0, limit - count - 1}`)

type SlidingWindowStore struct{ client *goredis.Client }

func NewSlidingWindowStore(client *goredis.Client) *SlidingWindowStore {
	return &SlidingWindowStore{client}
}
func (s *SlidingWindowStore) Allow(ctx context.Context, key string, limit int, window time.Duration) (globalratelimit.StoreDecision, error) {
	ms := window.Milliseconds()
	if limit <= 0 || ms <= 0 {
		return globalratelimit.StoreDecision{}, fmt.Errorf("sliding window: invalid limit or window")
	}
	member, err := rateLimitMember()
	if err != nil {
		return globalratelimit.StoreDecision{}, err
	}
	res, err := slidingWindowScript.Run(ctx, s.client, []string{slidingWindowKeyPrefix + key}, time.Now().UnixMilli(), ms, limit, member).Slice()
	if err != nil {
		return globalratelimit.StoreDecision{}, fmt.Errorf("sliding window allow: %w", err)
	}
	allowed, err := redisInt(res[0])
	if err != nil {
		return globalratelimit.StoreDecision{}, err
	}
	retry, err := redisInt(res[1])
	if err != nil {
		return globalratelimit.StoreDecision{}, err
	}
	remaining, err := redisInt(res[2])
	if err != nil {
		return globalratelimit.StoreDecision{}, err
	}
	d := globalratelimit.StoreDecision{Allowed: allowed == 1, Remaining: remaining}
	if !d.Allowed && retry > 0 {
		d.RetryAfter = time.Duration(retry) * time.Millisecond
	}
	return d, nil
}
func rateLimitMember() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d:%s", time.Now().UnixNano(), hex.EncodeToString(b[:])), nil
}
