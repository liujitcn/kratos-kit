// Package datadog 提供基于 DogStatsD 的指标实现。
package datadog

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/DataDog/datadog-go/v5/statsd"

	"github.com/liujitcn/kratos-kit/metrics"
)

var (
	_ metrics.Metrics = (*Provider)(nil)
	_ metrics.Closer  = (*Provider)(nil)
)

// Option 配置 Provider。
type Option func(*config)

type config struct {
	address               string
	namespace             string
	maxMessagesPerPayload int
	flushPeriod           time.Duration
	sampleRate            float64
}

// WithAddress 设置 DogStatsD Agent 地址。
func WithAddress(address string) Option {
	return func(config *config) {
		config.address = address
	}
}

// WithNamespace 设置所有指标名称的前缀。
func WithNamespace(namespace string) Option {
	return func(config *config) {
		config.namespace = namespace
	}
}

// WithBufferSize 设置单个 UDP payload 最多包含的指标消息数。
func WithBufferSize(size int) Option {
	return func(config *config) {
		if size > 0 {
			config.maxMessagesPerPayload = size
		}
	}
}

// WithFlushPeriod 设置缓冲指标的发送间隔。
func WithFlushPeriod(period time.Duration) Option {
	return func(config *config) {
		if period > 0 {
			config.flushPeriod = period
		}
	}
}

// WithSampleRate 设置采样率，合法范围为零到一。
func WithSampleRate(rate float64) Option {
	return func(config *config) {
		if rate >= 0 && rate <= 1 {
			config.sampleRate = rate
		}
	}
}

// Provider 使用 DogStatsD 记录指标。
type Provider struct {
	client     *statsd.Client
	sampleRate float64
}

// New 创建 Provider。
func New(options ...Option) (*Provider, error) {
	config := config{
		address:     "127.0.0.1:8125",
		flushPeriod: 100 * time.Millisecond,
		sampleRate:  1,
	}
	for _, option := range options {
		option(&config)
	}
	clientOptions := []statsd.Option{
		statsd.WithNamespace(config.namespace),
		statsd.WithBufferFlushInterval(config.flushPeriod),
	}
	if config.maxMessagesPerPayload > 0 {
		clientOptions = append(
			clientOptions,
			statsd.WithMaxMessagesPerPayload(config.maxMessagesPerPayload),
		)
	}
	client, err := statsd.New(config.address, clientOptions...)
	if err != nil {
		return nil, fmt.Errorf("create DogStatsD client: %w", err)
	}
	return &Provider{client: client, sampleRate: config.sampleRate}, nil
}

// Counter 实现 metrics.Metrics。
func (p *Provider) Counter(_ context.Context, name string, value int64, labels map[string]string) {
	_ = p.client.Count(name, value, tags(labels), p.sampleRate)
}

// Histogram 实现 metrics.Metrics。
func (p *Provider) Histogram(_ context.Context, name string, value float64, labels map[string]string) {
	_ = p.client.Histogram(name, value, tags(labels), p.sampleRate)
}

// Gauge 实现 metrics.Metrics。
func (p *Provider) Gauge(_ context.Context, name string, value float64, labels map[string]string) {
	_ = p.client.Gauge(name, value, tags(labels), p.sampleRate)
}

// Close 刷新并关闭 DogStatsD 客户端。
func (p *Provider) Close() error {
	return p.client.Close()
}

// tags 将标签转换为 DogStatsD 标签格式。
func tags(labels map[string]string) []string {
	names := make([]string, 0, len(labels))
	for name := range labels {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]string, 0, len(names))
	for _, name := range names {
		result = append(result, fmt.Sprintf("%s:%s", name, labels[name]))
	}
	return result
}
