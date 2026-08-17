package ratelimit

import (
	"context"
	"errors"
)

// ErrLimited 表示请求被限流器拒绝。
var ErrLimited = errors.New("rate limit exceeded")

// Limiter 定义传输层中间件使用的限流能力。
// Allow 用于立即尝试，Wait 用于等待直到允许或上下文取消。
type Limiter interface {
	Allow() (bool, error)
	Wait(context.Context) error
}
