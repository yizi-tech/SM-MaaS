package rate

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

// windowSecond is the sliding window length (in seconds) for RPM / TPM.
const windowSecond = int64(60)

// concurrentTTL bounds how long a concurrent counter may live without any
// acquire/release traffic. It bounds the leak window when a handler crashes
// mid-request: the counted slots vanish once the TTL elapses.
const concurrentTTL = 10 * time.Minute

// tpmScript implements a token-based sliding window:
//   - KEYS[1]: sorted set of "tokens:seq" members, score = unix second.
//   - Removes entries older than 60s, sums the remaining tokens, rejects the
//     request when sum + incoming tokens would exceed the limit, otherwise
//     records the new entry.
//
// The whole check-and-record runs atomically on the Redis server, so parallel
// requests on any number of instances cannot race the accounting.
var tpmScript = redis.NewScript(`
local now = tonumber(ARGV[1])
local tokens = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local member = ARGV[4]
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', now - 60)
local sum = 0
local entries = redis.call('ZRANGE', KEYS[1], 0, -1)
for _, e in ipairs(entries) do
  local sep = string.find(e, ':')
  if sep then
    sum = sum + tonumber(string.sub(e, 1, sep - 1))
  end
end
if sum + tokens > limit then
  return 0
end
redis.call('ZADD', KEYS[1], now, member)
redis.call('EXPIRE', KEYS[1], 120)
return 1
`)

// acquireScript atomically reserves one concurrent slot (INCR) and rolls it
// back when the new count would exceed the configured limit.
var acquireScript = redis.NewScript(`
local c = redis.call('INCR', KEYS[1])
if c > tonumber(ARGV[1]) then
  redis.call('DECR', KEYS[1])
  return 0
end
redis.call('EXPIRE', KEYS[1], tonumber(ARGV[2]))
return 1
`)

// releaseScript decrements a concurrent counter without going below zero and
// deletes it once it reaches zero (safe against double release).
var releaseScript = redis.NewScript(`
local c = redis.call('GET', KEYS[1])
if c and tonumber(c) > 0 then
  c = redis.call('DECR', KEYS[1])
  if tonumber(c) <= 0 then
    redis.call('DEL', KEYS[1])
  end
end
return 1
`)

// RateLimiter enforces RPM / TPM / concurrency limits in Redis, so limits are
// shared across all server instances. Every failure mode fails open: when
// Redis is unreachable the request proceeds and the error is logged.
type RateLimiter struct {
	client *redis.Client
	seq    atomic.Int64
}

func NewRateLimiter(client *redis.Client) *RateLimiter {
	return &RateLimiter{client: client}
}

type RateLimitConfig struct {
	RPM             int
	TPM             int
	ConcurrentLimit int
}

// nextMember returns a unique member string for a window entry.
func (rl *RateLimiter) nextMember() string {
	return fmt.Sprintf("%d", rl.seq.Add(1))
}

// CheckRPM checks if the request exceeds the RPM limit using a sliding window
// (sorted set of entry timestamps). Returns true when the request is allowed.
func (rl *RateLimiter) CheckRPM(ctx context.Context, key string, limit int) (bool, error) {
	if rl == nil || rl.client == nil || limit <= 0 {
		return true, nil
	}

	now := time.Now()
	windowKey := "rpm:" + key
	windowStart := now.Add(-time.Minute)

	pipe := rl.client.Pipeline()
	pipe.ZRemRangeByScore(ctx, windowKey, "0", fmt.Sprintf("%d", windowStart.Unix()))
	count := pipe.ZCard(ctx, windowKey)
	pipe.ZAdd(ctx, windowKey, redis.Z{Score: float64(now.Unix()), Member: rl.nextMember()})
	pipe.Expire(ctx, windowKey, 2*time.Minute)
	if _, err := pipe.Exec(ctx); err != nil {
		return false, err
	}

	return count.Val() < int64(limit), nil
}

// CheckTPM checks if consuming `tokens` would exceed the TPM limit within the
// sliding 60s window, and records them when allowed. Returns true when the
// request is allowed.
func (rl *RateLimiter) CheckTPM(ctx context.Context, key string, limit int, tokens int) (bool, error) {
	if rl == nil || rl.client == nil || limit <= 0 || tokens <= 0 {
		return true, nil
	}

	now := time.Now().Unix()
	windowKey := "tpm:" + key
	member := fmt.Sprintf("%d:%d", tokens, rl.seq.Add(1))
	res, err := tpmScript.Run(ctx, rl.client, []string{windowKey},
		now, tokens, limit, member).Int()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}

// AcquireConcurrent atomically reserves one concurrent request slot for the
// key. Returns true (with a slot held) when the current count is below the
// limit. The caller MUST pair it with ReleaseConcurrent once the request
// completes — including streaming requests, which hold the slot for the whole
// stream.
func (rl *RateLimiter) AcquireConcurrent(ctx context.Context, key string, limit int) (bool, error) {
	if rl == nil || rl.client == nil || limit <= 0 {
		return true, nil
	}

	windowKey := "conc:" + key
	res, err := acquireScript.Run(ctx, rl.client, []string{windowKey},
		limit, int(concurrentTTL.Seconds())).Int()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}

// ReleaseConcurrent releases a concurrent request slot reserved by
// AcquireConcurrent.
func (rl *RateLimiter) ReleaseConcurrent(ctx context.Context, key string) {
	if rl == nil || rl.client == nil {
		return
	}
	if _, err := releaseScript.Run(ctx, rl.client, []string{"conc:" + key}).Int(); err != nil {
		return
	}
}

// GetRateLimitKey generates a rate limit key for user+model
func GetRateLimitKey(userID uint, model string) string {
	return fmt.Sprintf("user:%d:model:%s", userID, model)
}

// GetAPIKeyRateLimitKey generates a rate limit key for API key
func GetAPIKeyRateLimitKey(keyID uint) string {
	return fmt.Sprintf("apikey:%d", keyID)
}
