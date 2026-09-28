package memory

import (
	"fmt"
	"math"
	"time"

	"github.com/liujitcn/kratos-kit/cache/store"
)

const rateLimitCleanupInterval = time.Minute

type rateLimitState struct {
	windowID     int64
	currentCount int
	previous     int
	events       []int64
	tokens       float64
	water        float64
	updatedAt    time.Time
	expiresAt    time.Time
}

// TakeTokenBuckets 原子判断并扣减一组令牌桶。
func (s *Memory) TakeTokenBuckets(requests []store.TokenBucketRequest) (bool, time.Duration, error) {
	rateLimits := make([]store.RateLimitRequest, len(requests))
	for index, request := range requests {
		rateLimits[index] = request.AsRateLimitRequest()
	}
	return s.TakeRateLimits(rateLimits)
}

// TakeRateLimits 在进程内原子判断并更新一组不同算法的限流状态。
func (s *Memory) TakeRateLimits(requests []store.RateLimitRequest) (bool, time.Duration, error) {
	if len(requests) == 0 {
		return true, 0, nil
	}
	now := time.Now()
	s.rateLimitMutex.Lock()
	defer s.rateLimitMutex.Unlock()
	if s.rateLimitItems == nil {
		s.rateLimitItems = make(map[string]rateLimitState)
	}
	if now.After(s.rateLimitCleanupAt) {
		for key, state := range s.rateLimitItems {
			if !now.Before(state.expiresAt) {
				delete(s.rateLimitItems, key)
			}
		}
		s.rateLimitCleanupAt = now.Add(rateLimitCleanupInterval)
	}
	updates := make(map[string]rateLimitState, len(requests))
	seen := make(map[string]struct{}, len(requests))
	var retryAfter time.Duration
	denied := false
	for _, request := range requests {
		if err := request.Validate(); err != nil {
			return false, 0, err
		}
		if _, exists := seen[request.Key]; exists {
			return false, 0, fmt.Errorf("duplicate rate limit key %q", request.Key)
		}
		seen[request.Key] = struct{}{}
		state := s.rateLimitItems[request.Key]
		state.events = append([]int64(nil), state.events...)
		if !state.expiresAt.IsZero() && !now.Before(state.expiresAt) {
			state = rateLimitState{}
		}
		allowed, retry, next := applyRateLimit(now, request, state)
		if !allowed {
			denied = true
			if retry > retryAfter {
				retryAfter = retry
			}
			continue
		}
		next.expiresAt = now.Add(rateLimitStateTTL(request))
		updates[request.Key] = next
	}
	if denied {
		return false, retryAfter, nil
	}
	for key, state := range updates {
		s.rateLimitItems[key] = state
	}
	return true, 0, nil
}

func applyRateLimit(now time.Time, request store.RateLimitRequest, state rateLimitState) (bool, time.Duration, rateLimitState) {
	switch request.Algorithm {
	case store.RateLimitAlgorithmTokenBucket:
		if state.updatedAt.IsZero() {
			state.tokens = float64(request.Burst)
		} else {
			state.tokens = math.Min(float64(request.Burst), state.tokens+now.Sub(state.updatedAt).Seconds()*request.TokensPerSecond)
		}
		if state.tokens < 1 {
			retry := time.Duration(math.Ceil((1-state.tokens)/request.TokensPerSecond*1000)) * time.Millisecond
			return false, minimumRateLimitRetry(retry), state
		}
		state.tokens--
		state.updatedAt = now
		return true, 0, state
	case store.RateLimitAlgorithmFixedWindow:
		windowID := now.UnixNano() / request.Window.Nanoseconds()
		if state.windowID != windowID {
			state = rateLimitState{windowID: windowID}
		}
		if state.currentCount+1 > request.Limit {
			return false, fixedWindowRetry(now, request.Window), state
		}
		state.currentCount++
		return true, 0, state
	case store.RateLimitAlgorithmSlidingWindowCounter:
		windowID := now.UnixNano() / request.Window.Nanoseconds()
		current := state.currentCount
		previous := state.previous
		if state.windowID != windowID {
			if state.windowID == windowID-1 {
				previous = current
			} else {
				previous = 0
			}
			current = 0
		}
		elapsed := time.Duration(now.UnixNano() % request.Window.Nanoseconds())
		weighted := float64(current) + float64(previous)*float64(request.Window-elapsed)/float64(request.Window)
		if weighted+1 > float64(request.Limit) {
			retry := fixedWindowRetry(now, request.Window)
			if current > 0 && current+1 > request.Limit {
				retry += time.Duration(math.Ceil(float64(current+1-request.Limit) * float64(request.Window) / float64(current)))
			}
			return false, minimumRateLimitRetry(retry), state
		}
		state.windowID = windowID
		state.currentCount = current + 1
		state.previous = previous
		return true, 0, state
	case store.RateLimitAlgorithmSlidingWindowLog:
		cutoff := now.Add(-request.Window).UnixNano()
		start := 0
		for start < len(state.events) && state.events[start] <= cutoff {
			start++
		}
		state.events = state.events[start:]
		if len(state.events) >= request.Limit {
			retry := time.Duration(state.events[0] + request.Window.Nanoseconds() - now.UnixNano())
			return false, minimumRateLimitRetry(retry), state
		}
		state.events = append(state.events, now.UnixNano())
		return true, 0, state
	case store.RateLimitAlgorithmLeakyBucket:
		if !state.updatedAt.IsZero() {
			state.water = math.Max(0, state.water-now.Sub(state.updatedAt).Seconds()*request.LeakRatePerSecond)
		}
		if state.water+1 > float64(request.Capacity) {
			retry := time.Duration(math.Ceil((state.water+1-float64(request.Capacity))/request.LeakRatePerSecond*1000)) * time.Millisecond
			return false, minimumRateLimitRetry(retry), state
		}
		state.water++
		state.updatedAt = now
		return true, 0, state
	default:
		return false, 0, state
	}
}

func rateLimitStateTTL(request store.RateLimitRequest) time.Duration {
	switch request.Algorithm {
	case store.RateLimitAlgorithmTokenBucket:
		return time.Duration(math.Ceil(float64(request.Burst)/request.TokensPerSecond*1000)+1000) * time.Millisecond
	case store.RateLimitAlgorithmFixedWindow, store.RateLimitAlgorithmSlidingWindowCounter:
		return request.Window * 2
	case store.RateLimitAlgorithmSlidingWindowLog:
		return request.Window
	case store.RateLimitAlgorithmLeakyBucket:
		return time.Duration(math.Ceil(float64(request.Capacity)/request.LeakRatePerSecond*1000)+1000) * time.Millisecond
	default:
		return time.Minute
	}
}

func fixedWindowRetry(now time.Time, window time.Duration) time.Duration {
	return minimumRateLimitRetry(window - time.Duration(now.UnixNano()%window.Nanoseconds()))
}

func minimumRateLimitRetry(retry time.Duration) time.Duration {
	if retry < time.Millisecond {
		return time.Millisecond
	}
	return retry
}
