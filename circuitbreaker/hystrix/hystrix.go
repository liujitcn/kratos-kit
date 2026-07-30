// Package hystrix 实现基于滑动窗口错误率的 Hystrix 风格熔断器。
package hystrix

import (
	"errors"
	"sync"
	"time"

	"github.com/liujitcn/kratos-kit/circuitbreaker"
)

const (
	defaultErrorThreshold         = 0.50
	defaultRequestVolumeThreshold = 20
	defaultSleepWindow            = 5 * time.Second
	defaultWindow                 = 10 * time.Second
	defaultBucketCount            = 10
)

// Option 配置 Hystrix 熔断器。
type Option func(*config)

type config struct {
	errorThreshold         float64
	requestVolumeThreshold int
	sleepWindow            time.Duration
	window                 time.Duration
	bucketCount            int
}

// WithErrorThreshold 设置触发熔断的错误率，取值范围为 (0, 1]。
func WithErrorThreshold(rate float64) Option {
	return func(c *config) {
		if rate > 0 && rate <= 1 {
			c.errorThreshold = rate
		}
	}
}

// WithRequestVolumeThreshold 设置开始计算错误率前的最小请求数。
func WithRequestVolumeThreshold(count int) Option {
	return func(c *config) {
		if count > 0 {
			c.requestVolumeThreshold = count
		}
	}
}

// WithSleepWindow 设置 Open 状态持续时间。
func WithSleepWindow(window time.Duration) Option {
	return func(c *config) {
		if window > 0 {
			c.sleepWindow = window
		}
	}
}

// WithWindow 设置统计滑动窗口长度。
func WithWindow(window time.Duration) Option {
	return func(c *config) {
		if window > 0 {
			c.window = window
		}
	}
}

// WithBucketCount 设置滑动窗口桶数量。
func WithBucketCount(count int) Option {
	return func(c *config) {
		if count > 0 {
			c.bucketCount = count
		}
	}
}

// New 创建 Hystrix 风格熔断器。
func New(opts ...Option) *Breaker {
	cfg := &config{
		errorThreshold:         defaultErrorThreshold,
		requestVolumeThreshold: defaultRequestVolumeThreshold,
		sleepWindow:            defaultSleepWindow,
		window:                 defaultWindow,
		bucketCount:            defaultBucketCount,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	bucketDuration := cfg.window / time.Duration(cfg.bucketCount)
	if bucketDuration <= 0 {
		bucketDuration = time.Nanosecond
	}
	now := time.Now()
	return &Breaker{
		cfg:            cfg,
		bucketDuration: bucketDuration,
		buckets:        make([]bucket, cfg.bucketCount),
		lastRotate:     now.Truncate(bucketDuration),
		state:          circuitbreaker.StateClosed,
		now:            time.Now,
	}
}

// Breaker 是 Hystrix 风格熔断器。
type Breaker struct {
	mu             sync.Mutex
	cfg            *config
	bucketDuration time.Duration
	buckets        []bucket
	lastRotate     time.Time
	state          circuitbreaker.State
	openedAt       time.Time
	probeInFlight  bool
	closed         bool
	now            func() time.Time
}

type bucket struct {
	requests int64
	failures int64
}

type permit struct {
	once     sync.Once
	breaker  *Breaker
	halfOpen bool
}

// Allow 判断请求是否可以执行，并返回请求级完成令牌。
func (b *Breaker) Allow() (circuitbreaker.Permit, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil, errors.New("hystrix: breaker is closed")
	}

	now := b.now()
	switch b.state {
	case circuitbreaker.StateOpen:
		if now.Sub(b.openedAt) < b.cfg.sleepWindow {
			return nil, errors.New("hystrix: circuit is open")
		}
		b.state = circuitbreaker.StateHalfOpen
	case circuitbreaker.StateHalfOpen:
	}

	halfOpen := b.state == circuitbreaker.StateHalfOpen
	if halfOpen {
		if b.probeInFlight {
			return nil, errors.New("hystrix: half-open probe is running")
		}
		b.probeInFlight = true
	}

	return &permit{breaker: b, halfOpen: halfOpen}, nil
}

// State 返回当前熔断状态。
func (b *Breaker) State() circuitbreaker.State {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.state == circuitbreaker.StateOpen &&
		b.now().Sub(b.openedAt) >= b.cfg.sleepWindow &&
		!b.probeInFlight {
		return circuitbreaker.StateHalfOpen
	}
	return b.state
}

// Close 关闭熔断器并拒绝后续请求。
func (b *Breaker) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	return nil
}

// Finish 记录本次请求结果。
func (p *permit) Finish(err error) {
	p.once.Do(func() {
		p.breaker.finish(p.halfOpen, err)
	})
}

// finish 更新滑动窗口和状态机。
func (b *Breaker) finish(halfOpen bool, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := b.now()
	if halfOpen {
		b.probeInFlight = false
		if err != nil {
			b.state = circuitbreaker.StateOpen
			b.openedAt = now
			return
		}
		b.state = circuitbreaker.StateClosed
		b.resetBucketsLocked()
		b.lastRotate = now.Truncate(b.bucketDuration)
		return
	}

	b.rotateLocked(now)
	index := b.bucketIndexLocked(now)
	b.buckets[index].requests++
	if err != nil {
		b.buckets[index].failures++
	}
	b.evaluateLocked(now)
}

// evaluateLocked 根据窗口错误率判断是否打开熔断器。
func (b *Breaker) evaluateLocked(now time.Time) {
	if b.state != circuitbreaker.StateClosed {
		return
	}

	var totalRequests int64
	var totalFailures int64
	for _, current := range b.buckets {
		totalRequests += current.requests
		totalFailures += current.failures
	}
	if totalRequests < int64(b.cfg.requestVolumeThreshold) {
		return
	}
	if float64(totalFailures)/float64(totalRequests) >= b.cfg.errorThreshold {
		b.state = circuitbreaker.StateOpen
		b.openedAt = now
	}
}

// rotateLocked 清理已经离开滑动窗口的桶。
func (b *Breaker) rotateLocked(now time.Time) {
	lastRotate := b.lastRotate.Truncate(b.bucketDuration)
	elapsed := now.Sub(lastRotate)
	steps := int(elapsed / b.bucketDuration)
	if steps <= 0 {
		b.lastRotate = lastRotate
		return
	}

	count := len(b.buckets)
	if steps >= count {
		b.resetBucketsLocked()
	} else {
		current := b.bucketIndexLocked(now)
		for offset := 0; offset < steps; offset++ {
			index := (current - offset + count) % count
			b.buckets[index] = bucket{}
		}
	}
	b.lastRotate = lastRotate.Add(time.Duration(steps) * b.bucketDuration)
}

// bucketIndexLocked 返回当前时间对应的桶下标。
func (b *Breaker) bucketIndexLocked(now time.Time) int {
	return int(now.UnixNano()/int64(b.bucketDuration)) % len(b.buckets)
}

// resetBucketsLocked 清空全部统计桶。
func (b *Breaker) resetBucketsLocked() {
	for index := range b.buckets {
		b.buckets[index] = bucket{}
	}
}
