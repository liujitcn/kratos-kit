package cache

import "github.com/liujitcn/kratos-kit/cache/store"

// ErrNotFound 表示缓存键不存在或已经过期。
var ErrNotFound = store.ErrNotFound

// Item 是现代缓存接口的批量写入项。
type Item = store.Item

// Store 是支持 context、GetDel、SetNX 和批量操作的缓存接口。
type Store = store.Store

// RateLimitAlgorithm 是支持的限流算法。
type RateLimitAlgorithm = store.RateLimitAlgorithm

// RateLimitRequest 描述一次限流状态判断及其算法参数。
type RateLimitRequest = store.RateLimitRequest

// RateLimitStore 是支持原子限流状态操作的缓存能力。
type RateLimitStore = store.RateLimitStore

const (
	RateLimitAlgorithmTokenBucket          = store.RateLimitAlgorithmTokenBucket
	RateLimitAlgorithmFixedWindow          = store.RateLimitAlgorithmFixedWindow
	RateLimitAlgorithmSlidingWindowCounter = store.RateLimitAlgorithmSlidingWindowCounter
	RateLimitAlgorithmSlidingWindowLog     = store.RateLimitAlgorithmSlidingWindowLog
	RateLimitAlgorithmLeakyBucket          = store.RateLimitAlgorithmLeakyBucket
	MaxSlidingWindowLogLimit               = store.MaxSlidingWindowLogLimit
)
