package redis

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/liujitcn/kratos-kit/cache/store"
	"github.com/redis/go-redis/v9"
)

const tokenBucketScript = `
local time = redis.call('TIME')
local now = tonumber(time[1]) * 1000 + math.floor(tonumber(time[2]) / 1000)
local count = #KEYS
local states = {}
local retry = 0
for i = 1, count do
  local rate = tonumber(ARGV[(i - 1) * 2 + 1])
  local burst = tonumber(ARGV[(i - 1) * 2 + 2])
  local values = redis.call('HMGET', KEYS[i], 'tokens', 'updated_at')
  local tokens = tonumber(values[1])
  local updated = tonumber(values[2])
  if not tokens or not updated then
    tokens = burst
    updated = now
  else
    tokens = math.min(burst, tokens + math.max(0, now - updated) * rate / 1000)
  end
  if tokens < 1 then
    retry = math.max(retry, math.ceil((1 - tokens) / rate * 1000))
  else
    tokens = tokens - 1
  end
  local ttl = math.ceil((burst / rate + 1) * 1000)
  states[i] = {tokens, now, ttl}
end
if retry > 0 then
  return {0, retry}
end
for i = 1, count do
  redis.call('HSET', KEYS[i], 'tokens', states[i][1], 'updated_at', states[i][2])
  redis.call('PEXPIRE', KEYS[i], states[i][3])
end
return {1, 0}
`

type scriptEvaluator interface {
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd
}

// takeTokenBuckets 使用 Lua 脚本原子判断并扣减多个令牌桶。
func takeTokenBuckets(ctx context.Context, client scriptEvaluator, requests []store.TokenBucketRequest, recordMeta func(string)) (bool, time.Duration, error) {
	if len(requests) == 0 {
		return true, 0, nil
	}
	keys := make([]string, len(requests))
	args := make([]interface{}, 0, len(requests)*2)
	for index, request := range requests {
		if request.Key == "" || request.TokensPerSecond <= 0 || request.Burst <= 0 || math.IsNaN(request.TokensPerSecond) || math.IsInf(request.TokensPerSecond, 0) {
			return false, 0, errors.New("invalid token bucket request")
		}
		keys[index] = request.Key
		args = append(args, request.TokensPerSecond, request.Burst)
	}
	result, err := client.Eval(ctx, tokenBucketScript, keys, args...).Slice()
	if err != nil {
		return false, 0, err
	}
	if len(result) != 2 {
		return false, 0, errors.New("invalid token bucket result")
	}
	allowed, ok := result[0].(int64)
	if !ok {
		return false, 0, errors.New("invalid token bucket decision")
	}
	retry, ok := result[1].(int64)
	if !ok {
		return false, 0, errors.New("invalid token bucket retry delay")
	}
	if allowed == 1 {
		for _, key := range keys {
			recordMeta(key)
		}
	}
	return allowed == 1, time.Duration(retry) * time.Millisecond, nil
}
