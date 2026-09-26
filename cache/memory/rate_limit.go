package memory

import (
	"errors"
	"math"
	"time"

	"github.com/liujitcn/kratos-kit/cache/store"
)

const tokenBucketCleanupInterval = time.Minute

type tokenBucketState struct {
	tokens    float64
	updatedAt time.Time
	expiresAt time.Time
}

// TakeTokenBuckets 原子判断并扣减同一请求的多个令牌桶。
func (s *Memory) TakeTokenBuckets(requests []store.TokenBucketRequest) (bool, time.Duration, error) {
	for _, request := range requests {
		if request.Key == "" || request.TokensPerSecond <= 0 || request.Burst <= 0 || math.IsNaN(request.TokensPerSecond) || math.IsInf(request.TokensPerSecond, 0) {
			return false, 0, errors.New("invalid token bucket request")
		}
	}
	if len(requests) == 0 {
		return true, 0, nil
	}
	s.tokenMutex.Lock()
	defer s.tokenMutex.Unlock()
	now := time.Now()
	if now.Sub(s.lastTokenBucketCleanup) >= tokenBucketCleanupInterval {
		for key, state := range s.tokenBuckets {
			if !now.Before(state.expiresAt) {
				delete(s.tokenBuckets, key)
			}
		}
		s.lastTokenBucketCleanup = now
	}
	updated := make([]tokenBucketState, len(requests))
	var retryAfter time.Duration
	for index, request := range requests {
		state, exists := s.tokenBuckets[request.Key]
		if !exists || !now.Before(state.expiresAt) {
			state = tokenBucketState{tokens: float64(request.Burst), updatedAt: now}
		}
		elapsed := now.Sub(state.updatedAt).Seconds()
		if elapsed < 0 {
			elapsed = 0
		}
		state.tokens = math.Min(float64(request.Burst), state.tokens+elapsed*request.TokensPerSecond)
		state.updatedAt = now
		if state.tokens < 1 {
			wait := time.Duration(math.Ceil((1 - state.tokens) / request.TokensPerSecond * float64(time.Second)))
			if wait > retryAfter {
				retryAfter = wait
			}
			continue
		}
		state.tokens--
		state.expiresAt = now.Add(time.Duration(math.Ceil((float64(request.Burst)/request.TokensPerSecond + 1) * float64(time.Second))))
		updated[index] = state
	}
	if retryAfter > 0 {
		return false, retryAfter, nil
	}
	for index, request := range requests {
		s.tokenBuckets[request.Key] = updated[index]
	}
	return true, 0, nil
}
