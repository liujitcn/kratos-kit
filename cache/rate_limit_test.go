package cache

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestMemoryRateLimitAlgorithms(t *testing.T) {
	storeValue, cleanup, err := NewCache(nil)
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}
	t.Cleanup(cleanup)
	store, ok := storeValue.(RateLimitStore)
	if !ok {
		t.Fatal("memory cache does not implement RateLimitStore")
	}
	tests := []struct {
		name    string
		request RateLimitRequest
		allowed []bool
	}{
		{
			name:    "token bucket",
			request: RateLimitRequest{Key: "token", Algorithm: RateLimitAlgorithmTokenBucket, TokensPerSecond: 0.001, Burst: 2},
			allowed: []bool{true, true, false},
		},
		{
			name:    "fixed window",
			request: RateLimitRequest{Key: "fixed", Algorithm: RateLimitAlgorithmFixedWindow, Limit: 2, Window: time.Second},
			allowed: []bool{true, true, false},
		},
		{
			name:    "sliding window counter",
			request: RateLimitRequest{Key: "counter", Algorithm: RateLimitAlgorithmSlidingWindowCounter, Limit: 2, Window: time.Second},
			allowed: []bool{true, true, false},
		},
		{
			name:    "sliding window log",
			request: RateLimitRequest{Key: "log", Algorithm: RateLimitAlgorithmSlidingWindowLog, Limit: 2, Window: time.Second},
			allowed: []bool{true, true, false},
		},
		{
			name:    "leaky bucket",
			request: RateLimitRequest{Key: "leaky", Algorithm: RateLimitAlgorithmLeakyBucket, LeakRatePerSecond: 0.001, Capacity: 2},
			allowed: []bool{true, true, false},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for index, want := range test.allowed {
				allowed, _, err := store.TakeRateLimits([]RateLimitRequest{test.request})
				if err != nil {
					t.Fatalf("TakeRateLimits() error = %v", err)
				}
				if allowed != want {
					t.Fatalf("request %d allowed = %t, want %t", index+1, allowed, want)
				}
			}
		})
	}
}

func TestMemoryRateLimitBatchIsAtomic(t *testing.T) {
	storeValue, cleanup, err := NewCache(nil)
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}
	t.Cleanup(cleanup)
	store := storeValue.(RateLimitStore)
	blocked := RateLimitRequest{Key: "blocked", Algorithm: RateLimitAlgorithmFixedWindow, Limit: 1, Window: time.Second}
	allowed, _, err := store.TakeRateLimits([]RateLimitRequest{blocked})
	if err != nil || !allowed {
		t.Fatalf("initial fixed-window request allowed = %t, error = %v", allowed, err)
	}
	newBucket := RateLimitRequest{Key: "new-token", Algorithm: RateLimitAlgorithmTokenBucket, TokensPerSecond: 0.001, Burst: 1}
	allowed, _, err = store.TakeRateLimits([]RateLimitRequest{newBucket, blocked})
	if err != nil || allowed {
		t.Fatalf("mixed request allowed = %t, error = %v, want denied", allowed, err)
	}
	allowed, _, err = store.TakeRateLimits([]RateLimitRequest{newBucket})
	if err != nil || !allowed {
		t.Fatalf("token was consumed by rejected batch: allowed = %t, error = %v", allowed, err)
	}
}

// TestMemoryRateLimitConcurrentRequestsRespectLimit 验证并发请求不会超过固定窗口配额。
func TestMemoryRateLimitConcurrentRequestsRespectLimit(t *testing.T) {
	storeValue, cleanup, err := NewCache(nil)
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}
	t.Cleanup(cleanup)
	store := storeValue.(RateLimitStore)
	const limit = 8
	const requests = 128
	var allowedCount atomic.Int64
	var waitGroup sync.WaitGroup
	errors := make(chan error, requests)
	waitGroup.Add(requests)
	for range requests {
		go func() {
			defer waitGroup.Done()
			allowed, _, err := store.TakeRateLimits([]RateLimitRequest{{
				Key:       "concurrent-fixed-window",
				Algorithm: RateLimitAlgorithmFixedWindow,
				Limit:     limit,
				Window:    time.Minute,
			}})
			if err != nil {
				errors <- err
			} else if allowed {
				allowedCount.Add(1)
			}
		}()
	}
	waitGroup.Wait()
	close(errors)
	for err := range errors {
		t.Errorf("TakeRateLimits() error = %v", err)
	}
	if got := allowedCount.Load(); got != limit {
		t.Fatalf("allowed concurrent requests = %d, want %d", got, limit)
	}
}

func TestRateLimitRequestValidation(t *testing.T) {
	request := RateLimitRequest{Key: "bad", Algorithm: RateLimitAlgorithmSlidingWindowLog, Limit: MaxSlidingWindowLogLimit + 1, Window: time.Second}
	if err := request.Validate(); err == nil {
		t.Fatal("Validate() accepted a sliding log limit above its memory bound")
	}
}
