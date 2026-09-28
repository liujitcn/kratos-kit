package redis

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/liujitcn/kratos-kit/cache/store"
	"github.com/redis/go-redis/v9"
)

const rateLimitScript = `
local time_parts = redis.call('TIME')
local now = tonumber(time_parts[1]) * 1000 + math.floor(tonumber(time_parts[2]) / 1000)
local count = #ARGV / 7
local states = {}
local max_retry = 0
for i = 1, count do
  local base = (i - 1) * 7
  local algorithm = ARGV[base + 1]
  local rate = tonumber(ARGV[base + 2])
  local burst = tonumber(ARGV[base + 3])
  local limit = tonumber(ARGV[base + 4])
  local window = tonumber(ARGV[base + 5])
  local leak_rate = tonumber(ARGV[base + 6])
  local capacity = tonumber(ARGV[base + 7])
  local key = KEYS[i]
  local retry = 0
  if algorithm == 'TOKEN_BUCKET' then
    local values = redis.call('HMGET', key, 'tokens', 'updated_at')
    local tokens = tonumber(values[1])
    local updated = tonumber(values[2])
    if not tokens or not updated then
      tokens = burst
    else
      tokens = math.min(burst, tokens + math.max(0, now - updated) * rate / 1000)
    end
    if tokens < 1 then
      retry = math.ceil((1 - tokens) / rate * 1000)
    else
      tokens = tokens - 1
    end
    states[i] = {algorithm, tokens, now, math.ceil((burst / rate + 1) * 1000)}
  elseif algorithm == 'FIXED_WINDOW' then
    local window_id = math.floor(now / window)
    local values = redis.call('HMGET', key, 'window_id', 'count')
    local current_window = tonumber(values[1])
    local current = tonumber(values[2]) or 0
    if current_window ~= window_id then current = 0 end
    if current + 1 > limit then
      retry = window - (now % window)
    else
      current = current + 1
    end
    states[i] = {algorithm, window_id, current, window * 2}
  elseif algorithm == 'SLIDING_WINDOW_COUNTER' then
    local window_id = math.floor(now / window)
    local elapsed = now % window
    local values = redis.call('HMGET', key, 'window_id', 'current', 'previous')
    local previous_window = tonumber(values[1])
    local current = tonumber(values[2]) or 0
    local previous = tonumber(values[3]) or 0
    if previous_window ~= window_id then
      if previous_window == window_id - 1 then previous = current else previous = 0 end
      current = 0
    end
    local weighted = current + previous * (window - elapsed) / window
    if weighted + 1 > limit then
      retry = window - elapsed
      if current > 0 and current + 1 > limit then
        retry = retry + math.ceil((current + 1 - limit) * window / current)
      end
    else
      current = current + 1
    end
    states[i] = {algorithm, window_id, current, previous, window * 2}
  elseif algorithm == 'SLIDING_WINDOW_LOG' then
    redis.call('ZREMRANGEBYSCORE', key, '-inf', now - window)
    local current = redis.call('ZCARD', key)
    if current >= limit then
      local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
      retry = tonumber(oldest[2]) + window - now
    end
    states[i] = {algorithm, now, window}
  elseif algorithm == 'LEAKY_BUCKET' then
    local values = redis.call('HMGET', key, 'water', 'updated_at')
    local water = tonumber(values[1]) or 0
    local updated = tonumber(values[2]) or now
    water = math.max(0, water - math.max(0, now - updated) * leak_rate / 1000)
    if water + 1 > capacity then
      retry = math.ceil((water + 1 - capacity) / leak_rate * 1000)
    else
      water = water + 1
    end
    states[i] = {algorithm, water, now, math.ceil(capacity / leak_rate * 1000) + 1000}
  else
    return redis.error_reply('unsupported rate limit algorithm')
  end
  if retry < 1 and states[i][1] == 'SLIDING_WINDOW_LOG' and redis.call('ZCARD', key) >= limit then retry = 1 end
  if retry > max_retry then max_retry = retry end
end
if max_retry > 0 then return {0, max_retry} end
for i = 1, count do
  local state = states[i]
  local key = KEYS[i]
  if state[1] == 'TOKEN_BUCKET' then
    redis.call('HSET', key, 'tokens', state[2], 'updated_at', state[3])
    redis.call('PEXPIRE', key, state[4])
  elseif state[1] == 'FIXED_WINDOW' then
    redis.call('HSET', key, 'window_id', state[2], 'count', state[3])
    redis.call('PEXPIRE', key, state[4])
  elseif state[1] == 'SLIDING_WINDOW_COUNTER' then
    redis.call('HSET', key, 'window_id', state[2], 'current', state[3], 'previous', state[4])
    redis.call('PEXPIRE', key, state[5])
  elseif state[1] == 'SLIDING_WINDOW_LOG' then
    local sequence = redis.call('INCR', KEYS[count + i])
    redis.call('ZADD', key, state[2], tostring(state[2]) .. ':' .. tostring(sequence))
    redis.call('PEXPIRE', key, state[3])
    redis.call('PEXPIRE', KEYS[count + i], state[3] * 2)
  elseif state[1] == 'LEAKY_BUCKET' then
    redis.call('HSET', key, 'water', state[2], 'updated_at', state[3])
    redis.call('PEXPIRE', key, state[4])
  end
end
return {1, 0}
`

type rateLimitScriptClient interface {
	Eval(context.Context, string, []string, ...interface{}) *redis.Cmd
}

// TakeRateLimits 原子判断并更新 Redis 中的一组限流算法状态。
func (s *Redis) TakeRateLimits(requests []store.RateLimitRequest) (bool, time.Duration, error) {
	return takeRateLimits(context.Background(), s.client, requests, false, s.recordMeta)
}

// TakeRateLimits 原子判断并更新 Redis 集群中的一组限流算法状态。
func (s *ClusterRedis) TakeRateLimits(requests []store.RateLimitRequest) (bool, time.Duration, error) {
	return takeRateLimits(context.Background(), s.client, requests, true, s.recordMeta)
}

func takeRateLimits(ctx context.Context, client rateLimitScriptClient, requests []store.RateLimitRequest, cluster bool, recordMeta func(string)) (bool, time.Duration, error) {
	if len(requests) == 0 {
		return true, 0, nil
	}
	if len(requests) > 100 {
		return false, 0, errors.New("too many rate limit requests")
	}
	keys := make([]string, 0, len(requests)*2)
	args := make([]interface{}, 0, len(requests)*7)
	seen := make(map[string]struct{}, len(requests))
	clusterTag := ""
	for _, request := range requests {
		if err := request.Validate(); err != nil {
			return false, 0, err
		}
		if _, exists := seen[request.Key]; exists {
			return false, 0, fmt.Errorf("duplicate rate limit key %q", request.Key)
		}
		seen[request.Key] = struct{}{}
		if cluster {
			tag, ok := rateLimitHashTag(request.Key)
			if !ok || clusterTag != "" && tag != clusterTag {
				return false, 0, errors.New("Redis Cluster rate limit keys must share a hash tag")
			}
			clusterTag = tag
		}
		keys = append(keys, request.Key)
		args = append(args,
			string(request.Algorithm),
			request.TokensPerSecond,
			request.Burst,
			request.Limit,
			request.Window.Milliseconds(),
			request.LeakRatePerSecond,
			request.Capacity,
		)
	}
	for _, request := range requests {
		keys = append(keys, request.Key+":sequence")
	}
	result, err := client.Eval(ctx, rateLimitScript, keys, args...).Int64Slice()
	if err != nil {
		return false, 0, err
	}
	if len(result) != 2 {
		return false, 0, errors.New("invalid rate limit script result")
	}
	allowed := result[0] == 1
	if allowed {
		for _, request := range requests {
			recordMeta(request.Key)
			if request.Algorithm == store.RateLimitAlgorithmSlidingWindowLog {
				recordMeta(request.Key + ":sequence")
			}
		}
	}
	return allowed, time.Duration(result[1]) * time.Millisecond, nil
}

func rateLimitHashTag(key string) (string, bool) {
	start := strings.IndexByte(key, '{')
	if start < 0 {
		return "", false
	}
	end := strings.IndexByte(key[start+1:], '}')
	if end <= 0 {
		return "", false
	}
	return key[start+1 : start+1+end], true
}
