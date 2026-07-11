package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	repokvstore "github.com/eannchen/go-backend-architecture/internal/repository/kvstore"
	goredis "github.com/redis/go-redis/v9"
)

const slidingWindowKeyPrefix = "ratelimit:sw:"

const slidingCore = `
local function admit(key, now_ms, window_ms, limit, member)
    redis.call('ZREMRANGEBYSCORE', key, '-inf', now_ms - window_ms)

    local count = redis.call('ZCARD', key)
    if count >= limit then
        local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
        local retry_ms = window_ms
        if oldest[2] ~= nil then
            retry_ms = tonumber(oldest[2]) + window_ms - now_ms
        end
        if retry_ms < 1 then retry_ms = 1 end
        return false, 0, retry_ms
    end

    redis.call('ZADD', key, now_ms, member)
    redis.call('PEXPIRE', key, window_ms)
    return true, limit - count - 1, 0
end`

var slidingWindowScript = goredis.NewScript(slidingCore + `
local admitted, remaining, retry_ms = admit(KEYS[1], tonumber(ARGV[1]), tonumber(ARGV[2]), tonumber(ARGV[3]), ARGV[4])
if admitted then
    return {1, 0, remaining}
end
return {0, retry_ms, 0}`)
var slidingWindowTieredScript = goredis.NewScript(slidingCore + `
local count = #KEYS
local now_ms = tonumber(ARGV[1])
local member = ARGV[2]

for i = 1, count do
    local admitted, _, retry_ms = admit(KEYS[i], now_ms, tonumber(ARGV[2 + count + i]), tonumber(ARGV[2 + i]), member)
    if not admitted then
        return {0, i, retry_ms}
    end
end

return {1, 0, 0}`)

type SlidingWindowStore struct{ client *goredis.Client }

func NewSlidingWindowStore(client *goredis.Client) *SlidingWindowStore {
	return &SlidingWindowStore{client}
}
func (s *SlidingWindowStore) Allow(ctx context.Context, key string, limit int, window time.Duration) (repokvstore.SlidingWindowDecision, error) {
	ms := window.Milliseconds()
	if limit <= 0 || ms <= 0 {
		return repokvstore.SlidingWindowDecision{}, fmt.Errorf("sliding window: invalid limit or window")
	}
	member, err := rateLimitMember()
	if err != nil {
		return repokvstore.SlidingWindowDecision{}, err
	}
	res, err := slidingWindowScript.Run(ctx, s.client, []string{slidingWindowKeyPrefix + key}, time.Now().UnixMilli(), ms, limit, member).Slice()
	if err != nil {
		return repokvstore.SlidingWindowDecision{}, fmt.Errorf("sliding window allow: %w", err)
	}
	allowed, err := redisInt(res[0])
	if err != nil {
		return repokvstore.SlidingWindowDecision{}, err
	}
	retry, err := redisInt(res[1])
	if err != nil {
		return repokvstore.SlidingWindowDecision{}, err
	}
	remaining, err := redisInt(res[2])
	if err != nil {
		return repokvstore.SlidingWindowDecision{}, err
	}
	d := repokvstore.SlidingWindowDecision{Allowed: allowed == 1, Remaining: remaining}
	if !d.Allowed && retry > 0 {
		d.RetryAfter = time.Duration(retry) * time.Millisecond
	}
	return d, nil
}
func (s *SlidingWindowStore) AllowTiered(ctx context.Context, tiers []repokvstore.SlidingWindowTier) (repokvstore.SlidingWindowTieredDecision, error) {
	if len(tiers) == 0 {
		return repokvstore.SlidingWindowTieredDecision{Allowed: true}, nil
	}
	keys := make([]string, len(tiers))
	args := make([]any, 0, 2+len(tiers)*2)
	member, err := rateLimitMember()
	if err != nil {
		return repokvstore.SlidingWindowTieredDecision{}, err
	}
	args = append(args, time.Now().UnixMilli(), member)
	for i, t := range tiers {
		if t.Limit <= 0 || t.Window.Milliseconds() <= 0 {
			return repokvstore.SlidingWindowTieredDecision{}, fmt.Errorf("sliding window: invalid tier %q", t.Key)
		}
		keys[i] = slidingWindowKeyPrefix + t.Key
		args = append(args, t.Limit)
	}
	for _, t := range tiers {
		args = append(args, t.Window.Milliseconds())
	}
	res, err := slidingWindowTieredScript.Run(ctx, s.client, keys, args...).Slice()
	if err != nil {
		return repokvstore.SlidingWindowTieredDecision{}, fmt.Errorf("sliding window tiered allow: %w", err)
	}
	allowed, err := redisInt(res[0])
	if err != nil {
		return repokvstore.SlidingWindowTieredDecision{}, err
	}
	if allowed == 1 {
		return repokvstore.SlidingWindowTieredDecision{Allowed: true}, nil
	}
	idx, err := redisInt(res[1])
	if err != nil {
		return repokvstore.SlidingWindowTieredDecision{}, err
	}
	retry, err := redisInt(res[2])
	if err != nil {
		return repokvstore.SlidingWindowTieredDecision{}, err
	}
	d := repokvstore.SlidingWindowTieredDecision{}
	if idx >= 1 && idx <= len(tiers) {
		d.RejectedTier = tiers[idx-1].Key
	}
	if retry > 0 {
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

var _ repokvstore.SlidingWindowRepository = (*SlidingWindowStore)(nil)
