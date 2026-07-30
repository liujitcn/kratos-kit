// Package tokenbucket 提供适用于 Kratos 服务端中间件的令牌桶限流器。
package tokenbucket

import (
	"errors"

	kratosRateLimit "github.com/go-kratos/kratos/v3/middleware/ratelimit"
	"golang.org/x/time/rate"
)

// ErrInvalidConfig 表示速率或突发容量无效。
var ErrInvalidConfig = errors.New("tokenbucket: rate and burst must be positive")

var _ kratosRateLimit.Limiter = (*Limiter)(nil)

// Limiter 使用 golang.org/x/time/rate 实现 Kratos 限流器。
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

// Allow 尝试消耗一个令牌，并返回 Kratos 请求完成回调。
func (l *Limiter) Allow() (kratosRateLimit.DoneFunc, error) {
	if !l.limiter.Allow() {
		return nil, kratosRateLimit.ErrLimitExceed
	}
	return func(kratosRateLimit.DoneInfo) {}, nil
}
