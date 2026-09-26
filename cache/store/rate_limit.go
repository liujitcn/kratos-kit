package store

// TokenBucketRequest 描述一次令牌桶扣减所需的键和参数。
type TokenBucketRequest struct {
	// Key 是令牌桶在缓存中的唯一键。
	Key string
	// TokensPerSecond 是令牌生成速率。
	TokensPerSecond float64
	// Burst 是令牌桶容量。
	Burst int
}
