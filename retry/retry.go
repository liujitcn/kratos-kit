// Package retry 提供与传输协议无关的重试能力。
package retry

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"sync"
	"time"
)

var (
	// ErrMaxAttempts 表示操作已用尽最大尝试次数。
	ErrMaxAttempts = errors.New("retry: max attempts exceeded")
	// ErrTimeout 表示操作超过了允许的总时长。
	ErrTimeout = errors.New("retry: total timeout exceeded")
)

// Backoff 计算一次失败后、下一次尝试前的等待时间。
type Backoff interface {
	Delay(attempt int) time.Duration
}

// Jitter 为等待时间增加随机抖动，避免大量请求同时重试。
type Jitter func(delay time.Duration, random func() float64) time.Duration

// Classifier 判断一个错误是否允许重试。
type Classifier func(err error) bool

// Retrier 保存重试配置，可由多个 goroutine 并发调用。
type Retrier struct {
	maxAttempts  int
	backoff      Backoff
	jitter       Jitter
	classifier   Classifier
	maxTotalWait time.Duration
	now          func() time.Time
	rng          *rand.Rand
	rngMu        sync.Mutex
}

// Option 配置 Retrier。
type Option func(*Retrier)

// WithMaxAttempts 设置最大尝试次数，包含第一次调用。
func WithMaxAttempts(attempts int) Option {
	return func(retrier *Retrier) {
		if attempts > 0 {
			retrier.maxAttempts = attempts
		}
	}
}

// WithBackoff 设置退避策略。
func WithBackoff(backoff Backoff) Option {
	return func(retrier *Retrier) {
		if backoff != nil {
			retrier.backoff = backoff
		}
	}
}

// WithJitter 设置等待时间的抖动策略。
func WithJitter(jitter Jitter) Option {
	return func(retrier *Retrier) {
		if jitter != nil {
			retrier.jitter = jitter
		}
	}
}

// WithClassifier 设置错误分类器，只有返回 true 的错误才会重试。
func WithClassifier(classifier Classifier) Option {
	return func(retrier *Retrier) {
		if classifier != nil {
			retrier.classifier = classifier
		}
	}
}

// WithMaxTotalWait 设置所有尝试和等待允许占用的总时长，零值表示不限制。
func WithMaxTotalWait(timeout time.Duration) Option {
	return func(retrier *Retrier) {
		if timeout > 0 {
			retrier.maxTotalWait = timeout
		}
	}
}

// WithRNG 设置随机源，主要用于需要确定性结果的测试。
func WithRNG(rng *rand.Rand) Option {
	return func(retrier *Retrier) {
		retrier.rng = rng
	}
}

// New 创建 Retrier。
func New(options ...Option) *Retrier {
	retrier := &Retrier{
		maxAttempts: 3,
		backoff: ExponentialBackoff{
			Initial: 200 * time.Millisecond,
			Factor:  2,
			Max:     10 * time.Second,
		},
		jitter:     NoJitter,
		classifier: RetryAny,
		now:        time.Now,
	}
	for _, option := range options {
		option(retrier)
	}
	return retrier
}

// Do 执行操作，并在可重试错误发生时按配置再次尝试。
func (r *Retrier) Do(ctx context.Context, operation func(context.Context) error) error {
	runCtx := ctx
	cancel := func() {}
	if r.maxTotalWait > 0 {
		var timeoutCancel context.CancelFunc
		runCtx, timeoutCancel = context.WithTimeout(ctx, r.maxTotalWait)
		cancel = timeoutCancel
	}
	defer cancel()

	var err error
	var lastErr error
	for attempt := 0; attempt < r.maxAttempts; attempt++ {
		err = runCtx.Err()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return errors.Join(ErrTimeout, lastErr)
		}

		lastErr = operation(runCtx)
		err = runCtx.Err()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return errors.Join(ErrTimeout, lastErr)
		}
		if lastErr == nil {
			return nil
		}
		if !r.classifier(lastErr) {
			return lastErr
		}
		if attempt == r.maxAttempts-1 {
			break
		}

		delay := r.jitter(r.backoff.Delay(attempt), r.random)
		if delay <= 0 {
			continue
		}
		timer := time.NewTimer(delay)
		select {
		case <-runCtx.Done():
			timer.Stop()
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return errors.Join(ErrTimeout, lastErr)
		case <-timer.C:
		}
	}
	return errors.Join(ErrMaxAttempts, lastErr)
}

// random 返回并发安全的随机小数。
func (r *Retrier) random() float64 {
	if r.rng == nil {
		return rand.Float64()
	}
	r.rngMu.Lock()
	defer r.rngMu.Unlock()
	return r.rng.Float64()
}

// ExponentialBackoff 按指数增长等待时间。
type ExponentialBackoff struct {
	// Initial 是第一次重试前的等待时间。
	Initial time.Duration
	// Factor 是每次重试的增长倍数。
	Factor float64
	// Max 是等待时间上限，零值表示不限制。
	Max time.Duration
}

// Delay 返回指定失败次数对应的指数退避时间。
func (b ExponentialBackoff) Delay(attempt int) time.Duration {
	if b.Initial <= 0 {
		return 0
	}
	if attempt < 0 {
		attempt = 0
	}
	factor := b.Factor
	if factor <= 0 {
		factor = 2
	}
	delay := float64(b.Initial) * math.Pow(factor, float64(attempt))
	if b.Max > 0 && delay >= float64(b.Max) {
		return b.Max
	}
	if math.IsInf(delay, 0) || delay >= float64(math.MaxInt64) {
		return time.Duration(math.MaxInt64)
	}
	return time.Duration(delay)
}

// FixedBackoff 表示固定等待时间。
type FixedBackoff time.Duration

// Delay 返回固定退避时间。
func (f FixedBackoff) Delay(_ int) time.Duration {
	return time.Duration(f)
}

// LinearBackoff 按固定步长增加等待时间。
type LinearBackoff struct {
	// Initial 是第一次重试前的等待时间。
	Initial time.Duration
	// Step 是每次重试增加的固定时长。
	Step time.Duration
	// Max 是等待时间上限，零值表示不限制。
	Max time.Duration
}

// Delay 返回指定失败次数对应的线性退避时间。
func (l LinearBackoff) Delay(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	delay := l.Initial + l.Step*time.Duration(attempt)
	if l.Max > 0 && delay > l.Max {
		return l.Max
	}
	return delay
}

// NoJitter 保持原等待时间不变。
func NoJitter(delay time.Duration, _ func() float64) time.Duration {
	return delay
}

// FullJitter 返回零到原等待时间之间的随机值。
func FullJitter(delay time.Duration, random func() float64) time.Duration {
	return time.Duration(float64(delay) * random())
}

// EqualJitter 至少保留一半等待时间，并对另一半增加随机抖动。
func EqualJitter(delay time.Duration, random func() float64) time.Duration {
	half := delay / 2
	return half + time.Duration(float64(half)*random())
}

// RetryAny 对任意错误执行重试。
func RetryAny(error) bool {
	return true
}

// RetryNever 不对任何错误执行重试。
func RetryNever(error) bool {
	return false
}

// RetryIf 将调用方提供的判断函数转换为 Classifier。
func RetryIf(predicate func(error) bool) Classifier {
	return predicate
}
