package sentinel

import (
	"errors"
	"sync"

	sentinelapi "github.com/alibaba/sentinel-golang/api"
	"github.com/alibaba/sentinel-golang/core/base"

	"github.com/liujitcn/kratos-kit/circuitbreaker"
)

// Option 配置 Sentinel 熔断器。
type Option func(*config)

type config struct {
	trafficType base.TrafficType
	entryOpts   []sentinelapi.EntryOption
}

// WithTrafficType 设置 Sentinel 流量类型。
func WithTrafficType(trafficType base.TrafficType) Option {
	return func(c *config) {
		c.trafficType = trafficType
	}
}

// WithEntryOptions 设置 Sentinel Entry 附加选项。
func WithEntryOptions(opts ...sentinelapi.EntryOption) Option {
	return func(c *config) {
		c.entryOpts = append(c.entryOpts, opts...)
	}
}

// New 创建指定资源的 Sentinel 熔断器。
func New(resource string, opts ...Option) *Breaker {
	cfg := &config{trafficType: base.Outbound}
	for _, opt := range opts {
		opt(cfg)
	}
	return &Breaker{
		resource: resource,
		cfg:      cfg,
	}
}

// Breaker 将 Sentinel Entry 生命周期绑定到请求级 Permit。
type Breaker struct {
	resource string
	cfg      *config
}

type permit struct {
	once  sync.Once
	entry *base.SentinelEntry
}

// Allow 创建本次请求独立的 Sentinel Entry。
func (b *Breaker) Allow() (circuitbreaker.Permit, error) {
	opts := []sentinelapi.EntryOption{
		sentinelapi.WithTrafficType(b.cfg.trafficType),
	}
	opts = append(opts, b.cfg.entryOpts...)

	entry, blockErr := sentinelapi.Entry(b.resource, opts...)
	if blockErr != nil {
		return nil, errors.New("sentinel: circuit is open")
	}
	return &permit{entry: entry}, nil
}

// Finish 记录真实请求错误并关闭对应的 Sentinel Entry。
func (p *permit) Finish(err error) {
	p.once.Do(func() {
		if err != nil {
			sentinelapi.TraceError(p.entry, err)
		}
		p.entry.Exit()
	})
}
