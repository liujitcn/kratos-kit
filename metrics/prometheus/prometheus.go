// Package prometheus 提供基于 Prometheus client_golang 的指标实现。
package prometheus

import (
	"context"
	"errors"
	"sort"
	"sync"

	prom "github.com/prometheus/client_golang/prometheus"

	"github.com/liujitcn/kratos-kit/metrics"
)

var _ metrics.Metrics = (*Provider)(nil)

// Option 配置 Provider。
type Option func(*config)

type config struct {
	namespace string
	subsystem string
	registry  prom.Registerer
}

type counter struct {
	labels []string
	vector *prom.CounterVec
}

type histogram struct {
	labels []string
	vector *prom.HistogramVec
}

type gauge struct {
	labels []string
	vector *prom.GaugeVec
}

// WithNamespace 设置指标命名空间。
func WithNamespace(namespace string) Option {
	return func(config *config) {
		config.namespace = namespace
	}
}

// WithSubsystem 设置指标子系统。
func WithSubsystem(subsystem string) Option {
	return func(config *config) {
		config.subsystem = subsystem
	}
}

// WithRegistry 设置自定义 Prometheus 注册器。
func WithRegistry(registry prom.Registerer) Option {
	return func(config *config) {
		if registry != nil {
			config.registry = registry
		}
	}
}

// Provider 使用 Prometheus 记录指标。
type Provider struct {
	namespace  string
	subsystem  string
	registerer prom.Registerer
	gatherer   prom.Gatherer

	mu         sync.Mutex
	counters   map[string]counter
	histograms map[string]histogram
	gauges     map[string]gauge
}

// New 创建使用独立 Registry 的 Provider。
func New(options ...Option) (*Provider, error) {
	registry := prom.NewRegistry()
	return newProvider(registry, registry, options...), nil
}

// NewWithDefaultRegistry 创建默认注册到进程全局 Registry 的 Provider。
func NewWithDefaultRegistry(options ...Option) (*Provider, error) {
	return newProvider(prom.DefaultRegisterer, prom.DefaultGatherer, options...), nil
}

// Registry 返回 Provider 对应的指标采集器。
func (p *Provider) Registry() prom.Gatherer {
	return p.gatherer
}

// Counter 实现 metrics.Metrics。
func (p *Provider) Counter(_ context.Context, name string, value int64, labels map[string]string) {
	p.mu.Lock()
	instrument, ok := p.counters[name]
	if !ok {
		labelNames := sortedLabelNames(labels)
		vector := prom.NewCounterVec(prom.CounterOpts{
			Namespace: p.namespace,
			Subsystem: p.subsystem,
			Name:      name,
		}, labelNames)
		collector, registered := p.register(vector)
		if !registered {
			p.mu.Unlock()
			return
		}
		var typeOK bool
		vector, typeOK = collector.(*prom.CounterVec)
		if !typeOK {
			p.mu.Unlock()
			return
		}
		instrument = counter{labels: labelNames, vector: vector}
		p.counters[name] = instrument
	}
	if !sameLabels(instrument.labels, labels) {
		p.mu.Unlock()
		return
	}
	p.mu.Unlock()
	var metric prom.Counter
	var err error
	metric, err = instrument.vector.GetMetricWith(labels)
	if err == nil {
		metric.Add(float64(value))
	}
}

// Histogram 实现 metrics.Metrics。
func (p *Provider) Histogram(_ context.Context, name string, value float64, labels map[string]string) {
	p.mu.Lock()
	instrument, ok := p.histograms[name]
	if !ok {
		labelNames := sortedLabelNames(labels)
		vector := prom.NewHistogramVec(prom.HistogramOpts{
			Namespace: p.namespace,
			Subsystem: p.subsystem,
			Name:      name,
		}, labelNames)
		collector, registered := p.register(vector)
		if !registered {
			p.mu.Unlock()
			return
		}
		var typeOK bool
		vector, typeOK = collector.(*prom.HistogramVec)
		if !typeOK {
			p.mu.Unlock()
			return
		}
		instrument = histogram{labels: labelNames, vector: vector}
		p.histograms[name] = instrument
	}
	if !sameLabels(instrument.labels, labels) {
		p.mu.Unlock()
		return
	}
	p.mu.Unlock()
	var metric prom.Observer
	var err error
	metric, err = instrument.vector.GetMetricWith(labels)
	if err == nil {
		metric.Observe(value)
	}
}

// Gauge 实现 metrics.Metrics。
func (p *Provider) Gauge(_ context.Context, name string, value float64, labels map[string]string) {
	p.mu.Lock()
	instrument, ok := p.gauges[name]
	if !ok {
		labelNames := sortedLabelNames(labels)
		vector := prom.NewGaugeVec(prom.GaugeOpts{
			Namespace: p.namespace,
			Subsystem: p.subsystem,
			Name:      name,
		}, labelNames)
		collector, registered := p.register(vector)
		if !registered {
			p.mu.Unlock()
			return
		}
		var typeOK bool
		vector, typeOK = collector.(*prom.GaugeVec)
		if !typeOK {
			p.mu.Unlock()
			return
		}
		instrument = gauge{labels: labelNames, vector: vector}
		p.gauges[name] = instrument
	}
	if !sameLabels(instrument.labels, labels) {
		p.mu.Unlock()
		return
	}
	p.mu.Unlock()
	var metric prom.Gauge
	var err error
	metric, err = instrument.vector.GetMetricWith(labels)
	if err == nil {
		metric.Set(value)
	}
}

// newProvider 使用指定的默认注册器创建 Provider。
func newProvider(
	defaultRegisterer prom.Registerer,
	defaultGatherer prom.Gatherer,
	options ...Option,
) *Provider {
	config := &config{registry: defaultRegisterer}
	for _, option := range options {
		option(config)
	}
	gatherer := defaultGatherer
	if customGatherer, ok := config.registry.(prom.Gatherer); ok {
		gatherer = customGatherer
	}
	return &Provider{
		namespace:  config.namespace,
		subsystem:  config.subsystem,
		registerer: config.registry,
		gatherer:   gatherer,
		counters:   make(map[string]counter),
		histograms: make(map[string]histogram),
		gauges:     make(map[string]gauge),
	}
}

// register 注册 collector，并复用已经存在的同类型 collector。
func (p *Provider) register(collector prom.Collector) (prom.Collector, bool) {
	err := p.registerer.Register(collector)
	if err == nil {
		return collector, true
	}
	var alreadyRegistered prom.AlreadyRegisteredError
	if errors.As(err, &alreadyRegistered) {
		return alreadyRegistered.ExistingCollector, true
	}
	return nil, false
}

// sortedLabelNames 返回稳定排序的标签名称。
func sortedLabelNames(labels map[string]string) []string {
	names := make([]string, 0, len(labels))
	for name := range labels {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// sameLabels 判断本次标签集合是否与指标首次注册时一致。
func sameLabels(names []string, labels map[string]string) bool {
	if len(names) != len(labels) {
		return false
	}
	for _, name := range names {
		if _, ok := labels[name]; !ok {
			return false
		}
	}
	return true
}
