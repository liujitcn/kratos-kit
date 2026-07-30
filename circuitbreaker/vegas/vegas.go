// Package vegas 实现基于请求延迟膨胀的 Vegas 风格熔断器。
package vegas

import (
	"errors"
	"math"
	"sync"
	"time"

	"github.com/liujitcn/kratos-kit/circuitbreaker"
)

const (
	defaultAlpha         = 0.5
	defaultBeta          = 0.3
	defaultWarmupSamples = 10
	defaultMinRTT        = time.Millisecond
	defaultMaxRTT        = 30 * time.Second
	defaultRetryTimeout  = 5 * time.Second
)

// Option 配置 Vegas 熔断器。
type Option func(*config)

type config struct {
	alpha         float64
	beta          float64
	warmupSamples int
	minRTT        time.Duration
	maxRTT        time.Duration
	retryTimeout  time.Duration
}

// WithAlpha 设置触发熔断的延迟膨胀比例。
func WithAlpha(alpha float64) Option {
	return func(c *config) {
		if alpha > 0 {
			c.alpha = alpha
		}
	}
}

// WithBeta 设置恢复阈值，必须小于 Alpha。
func WithBeta(beta float64) Option {
	return func(c *config) {
		if beta > 0 {
			c.beta = beta
		}
	}
}

// WithWarmupSamples 设置开始判断状态前的样本数。
func WithWarmupSamples(count int) Option {
	return func(c *config) {
		if count > 0 {
			c.warmupSamples = count
		}
	}
}

// WithRetryTimeout 设置打开熔断后执行恢复探测前的等待时间。
func WithRetryTimeout(timeout time.Duration) Option {
	return func(c *config) {
		if timeout > 0 {
			c.retryTimeout = timeout
		}
	}
}

// New 创建 Vegas 风格熔断器。
func New(opts ...Option) *Breaker {
	cfg := &config{
		alpha:         defaultAlpha,
		beta:          defaultBeta,
		warmupSamples: defaultWarmupSamples,
		minRTT:        defaultMinRTT,
		maxRTT:        defaultMaxRTT,
		retryTimeout:  defaultRetryTimeout,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	if cfg.beta >= cfg.alpha {
		cfg.beta = cfg.alpha / 2
	}

	return &Breaker{
		cfg:   cfg,
		state: circuitbreaker.StateClosed,
		now:   time.Now,
	}
}

// Breaker 是 Vegas 风格熔断器。
type Breaker struct {
	mu            sync.Mutex
	cfg           *config
	baseRTT       time.Duration
	currentRTT    time.Duration
	sampleCount   int
	failureCount  int
	state         circuitbreaker.State
	openedAt      time.Time
	probeInFlight bool
	closed        bool
	now           func() time.Time
}

type permit struct {
	once     sync.Once
	breaker  *Breaker
	started  time.Time
	halfOpen bool
}

// Allow 判断请求是否可以执行，并开始记录真实请求耗时。
func (b *Breaker) Allow() (circuitbreaker.Permit, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil, errors.New("vegas: breaker is closed")
	}

	now := b.now()
	switch b.state {
	case circuitbreaker.StateOpen:
		if now.Sub(b.openedAt) < b.cfg.retryTimeout {
			return nil, errors.New("vegas: circuit is open")
		}
		b.state = circuitbreaker.StateHalfOpen
	case circuitbreaker.StateHalfOpen:
	}

	halfOpen := b.state == circuitbreaker.StateHalfOpen
	if halfOpen {
		if b.probeInFlight {
			return nil, errors.New("vegas: half-open probe is running")
		}
		b.probeInFlight = true
	}
	return &permit{
		breaker:  b,
		started:  now,
		halfOpen: halfOpen,
	}, nil
}

// RecordLatency 直接记录一个外部延迟样本。
func (b *Breaker) RecordLatency(rtt time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failureCount = 0
	b.updateRTTLocked(rtt)
	b.evaluateLocked(b.now())
}

// State 返回当前熔断状态。
func (b *Breaker) State() circuitbreaker.State {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.state == circuitbreaker.StateOpen &&
		b.now().Sub(b.openedAt) >= b.cfg.retryTimeout &&
		!b.probeInFlight {
		return circuitbreaker.StateHalfOpen
	}
	return b.state
}

// BaseRTT 返回当前基准延迟。
func (b *Breaker) BaseRTT() time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.baseRTT
}

// CurrentRTT 返回当前平滑延迟。
func (b *Breaker) CurrentRTT() time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.currentRTT
}

// Inflation 返回当前延迟相对基准值的膨胀比例。
func (b *Breaker) Inflation() float64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.baseRTT == 0 {
		return 0
	}
	return math.Max(0, float64(b.currentRTT-b.baseRTT)/float64(b.baseRTT))
}

// Close 关闭熔断器并拒绝后续请求。
func (b *Breaker) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	return nil
}

// Finish 使用请求真实耗时和结果更新熔断器。
func (p *permit) Finish(err error) {
	p.once.Do(func() {
		p.breaker.finish(p.started, p.halfOpen, err)
	})
}

// finish 更新延迟样本和恢复探测状态。
func (b *Breaker) finish(started time.Time, halfOpen bool, err error) {
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

		rtt := now.Sub(started)
		if rtt < b.cfg.minRTT {
			rtt = b.cfg.minRTT
		}
		if rtt > b.cfg.maxRTT {
			b.state = circuitbreaker.StateOpen
			b.openedAt = now
			return
		}
		if b.baseRTT > 0 {
			inflation := math.Max(0, float64(rtt-b.baseRTT)/float64(b.baseRTT))
			if inflation > b.cfg.beta {
				b.state = circuitbreaker.StateOpen
				b.openedAt = now
				return
			}
		}

		// 恢复后的服务可能具有新的稳定延迟，探测成功后以该样本重建基线。
		b.baseRTT = rtt
		b.currentRTT = rtt
		b.sampleCount = 1
		b.failureCount = 0
		b.state = circuitbreaker.StateClosed
		return
	}

	if err != nil {
		if b.baseRTT == 0 {
			b.failureCount++
			if b.failureCount >= b.cfg.warmupSamples {
				b.state = circuitbreaker.StateOpen
				b.openedAt = now
			}
			return
		}
		b.updateRTTLocked(b.cfg.maxRTT)
	} else {
		b.failureCount = 0
		b.updateRTTLocked(now.Sub(started))
	}
	b.evaluateLocked(now)
}

// updateRTTLocked 更新基准延迟和平滑延迟。
func (b *Breaker) updateRTTLocked(rtt time.Duration) {
	if rtt < b.cfg.minRTT || rtt > b.cfg.maxRTT {
		return
	}

	b.sampleCount++
	if b.currentRTT == 0 {
		b.currentRTT = rtt
	} else {
		b.currentRTT = time.Duration(float64(b.currentRTT)*0.875 + float64(rtt)*0.125)
	}
	if b.baseRTT == 0 || rtt < b.baseRTT {
		b.baseRTT = rtt
	}
}

// evaluateLocked 根据延迟膨胀比例更新状态。
func (b *Breaker) evaluateLocked(now time.Time) {
	if b.state != circuitbreaker.StateClosed ||
		b.sampleCount < b.cfg.warmupSamples ||
		b.baseRTT == 0 {
		return
	}

	inflation := float64(b.currentRTT-b.baseRTT) / float64(b.baseRTT)
	if inflation > b.cfg.alpha {
		b.state = circuitbreaker.StateOpen
		b.openedAt = now
	}
}
