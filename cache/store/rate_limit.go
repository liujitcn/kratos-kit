package store

import (
	"fmt"
	"math"
	"time"
)

// RateLimitAlgorithm 表示缓存层支持的限流算法。
type RateLimitAlgorithm string

const (
	// RateLimitAlgorithmTokenBucket 表示允许突发流量的令牌桶。
	RateLimitAlgorithmTokenBucket RateLimitAlgorithm = "TOKEN_BUCKET"
	// RateLimitAlgorithmFixedWindow 表示按固定时间窗计数的限流算法。
	RateLimitAlgorithmFixedWindow RateLimitAlgorithm = "FIXED_WINDOW"
	// RateLimitAlgorithmSlidingWindowCounter 表示使用相邻窗口加权计数的算法。
	RateLimitAlgorithmSlidingWindowCounter RateLimitAlgorithm = "SLIDING_WINDOW_COUNTER"
	// RateLimitAlgorithmSlidingWindowLog 表示按请求时间记录精确滑动窗口的算法。
	RateLimitAlgorithmSlidingWindowLog RateLimitAlgorithm = "SLIDING_WINDOW_LOG"
	// RateLimitAlgorithmLeakyBucket 表示拒绝溢出请求的漏桶算法。
	RateLimitAlgorithmLeakyBucket RateLimitAlgorithm = "LEAKY_BUCKET"
)

// MaxSlidingWindowLogLimit 限制精确滑动窗口日志可保存的请求数。
const MaxSlidingWindowLogLimit = 10000

const (
	maxRateLimitRate     = 1000000.0
	maxRateLimitCapacity = 1000000
	maxRateLimitWindow   = 24 * time.Hour
)

// TokenBucketRequest 描述一次令牌桶扣减所需的键和参数。
type TokenBucketRequest struct {
	// Key 是令牌桶在缓存中的唯一键。
	Key string
	// TokensPerSecond 是令牌生成速率。
	TokensPerSecond float64
	// Burst 是令牌桶容量。
	Burst int
}

// RateLimitRequest 描述一次限流状态判断及其算法参数。
type RateLimitRequest struct {
	// Key 是该身份维度对应的唯一状态键。
	Key string
	// Algorithm 是本次状态操作使用的算法。
	Algorithm RateLimitAlgorithm
	// TokensPerSecond 是令牌桶每秒生成的令牌数。
	TokensPerSecond float64
	// Burst 是令牌桶最大容量。
	Burst int
	// Limit 是固定或滑动窗口内允许的请求数。
	Limit int
	// Window 是固定或滑动窗口的时间长度。
	Window time.Duration
	// LeakRatePerSecond 是漏桶每秒漏出的水量。
	LeakRatePerSecond float64
	// Capacity 是漏桶最大容量。
	Capacity int
}

// AsRateLimitRequest 将令牌桶请求转换为统一限流请求。
func (request TokenBucketRequest) AsRateLimitRequest() RateLimitRequest {
	return RateLimitRequest{
		Key:             request.Key,
		Algorithm:       RateLimitAlgorithmTokenBucket,
		TokensPerSecond: request.TokensPerSecond,
		Burst:           request.Burst,
	}
}

// RateLimitStore 定义原子判断并更新一组限流状态的缓存能力。
type RateLimitStore interface {
	// TakeRateLimits 原子判断并更新一组限流状态；有任一项超限时不消费任何项。
	TakeRateLimits([]RateLimitRequest) (bool, time.Duration, error)
}

// Validate 检查限流请求的键和当前算法参数。
func (request RateLimitRequest) Validate() error {
	if request.Key == "" {
		return fmt.Errorf("rate limit key is required")
	}
	switch request.Algorithm {
	case RateLimitAlgorithmTokenBucket:
		if !validRate(request.TokensPerSecond) || request.Burst < 1 || request.Burst > maxRateLimitCapacity {
			return fmt.Errorf("invalid token bucket parameters")
		}
	case RateLimitAlgorithmFixedWindow, RateLimitAlgorithmSlidingWindowCounter:
		if request.Limit < 1 || request.Limit > maxRateLimitCapacity || !validWindow(request.Window) {
			return fmt.Errorf("invalid window counter parameters")
		}
	case RateLimitAlgorithmSlidingWindowLog:
		if request.Limit < 1 || request.Limit > MaxSlidingWindowLogLimit || !validWindow(request.Window) {
			return fmt.Errorf("invalid sliding window log parameters")
		}
	case RateLimitAlgorithmLeakyBucket:
		if !validRate(request.LeakRatePerSecond) || request.Capacity < 1 || request.Capacity > maxRateLimitCapacity {
			return fmt.Errorf("invalid leaky bucket parameters")
		}
	default:
		return fmt.Errorf("unsupported rate limit algorithm %q", request.Algorithm)
	}
	return nil
}

func validRate(rate float64) bool {
	return rate >= 0.001 && rate <= maxRateLimitRate && !math.IsNaN(rate) && !math.IsInf(rate, 0)
}

func validWindow(window time.Duration) bool {
	return window >= time.Millisecond && window <= maxRateLimitWindow
}
