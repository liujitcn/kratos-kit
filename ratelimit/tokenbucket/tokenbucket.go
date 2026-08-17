package tokenbucket

import (
	"context"
	"errors"

	"golang.org/x/time/rate"
)

// ErrInvalidConfig 表示速率或突发容量无效。
var ErrInvalidConfig = errors.New("tokenbucket: rate and burst must be positive")

// ErrLimited 表示没有可用令牌。
var ErrLimited = errors.New("rate limit exceeded")

// Limiter 使用 golang.org/x/time/rate 实现限流器。
type Limiter struct {
	limiter *rate.Limiter
}

// New 创建每秒补充 tokensPerSecond 个令牌、容量为 burst 的限流器。
func New(tokensPerSecond float64, burst int) (*Limiter, error) {
	if tokensPerSecond <= 0 || burst <= 0 {
		return nil, ErrInvalidConfig
	}
	return &Limiter{
		limiter: rate.NewLimiter(rate.Limit(tokensPerSecond), burst),
	}, nil
}

// Allow 尝试立即消耗一个令牌。
func (l *Limiter) Allow() (bool, error) {
	if l.limiter.Allow() {
		return true, nil
	}
	return false, ErrLimited
}

// Wait 等待直到获得一个令牌或上下文取消。
func (l *Limiter) Wait(ctx context.Context) error {
	return l.limiter.Wait(ctx)
}
