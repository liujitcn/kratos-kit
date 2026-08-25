package otel

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"

	"github.com/liujitcn/kratos-kit/metrics"
)

const (
	defaultGRPCEndpoint = "localhost:4317"
	defaultHTTPEndpoint = "localhost:4318"
)

var (
	_ metrics.Metrics = (*Provider)(nil)
	_ metrics.Closer  = (*Provider)(nil)
)

// Option 配置 Provider。
type Option func(*config)

type config struct {
	endpoint       string
	serviceName    string
	serviceVersion string
	insecure       bool
	useHTTP        bool
	exportInterval time.Duration
}

// WithEndpoint 设置 OTLP collector 地址。
func WithEndpoint(endpoint string) Option {
	return func(config *config) {
		config.endpoint = endpoint
	}
}

// WithServiceName 设置资源中的服务名。
func WithServiceName(name string) Option {
	return func(config *config) {
		config.serviceName = name
	}
}

// WithServiceVersion 设置资源中的服务版本。
func WithServiceVersion(version string) Option {
	return func(config *config) {
		config.serviceVersion = version
	}
}

// WithInsecure 设置是否使用明文 OTLP 连接。
func WithInsecure(insecure bool) Option {
	return func(config *config) {
		config.insecure = insecure
	}
}

// WithHTTP 设置是否使用 OTLP HTTP 协议，默认使用 gRPC。
func WithHTTP(useHTTP bool) Option {
	return func(config *config) {
		config.useHTTP = useHTTP
	}
}

// WithExportInterval 设置周期导出间隔。
func WithExportInterval(interval time.Duration) Option {
	return func(config *config) {
		if interval > 0 {
			config.exportInterval = interval
		}
	}
}

// Provider 使用 OpenTelemetry MeterProvider 记录指标。
type Provider struct {
	meter         metric.Meter
	meterProvider *sdkmetric.MeterProvider

	mu         sync.Mutex
	counters   map[string]metric.Int64Counter
	histograms map[string]metric.Float64Histogram
	gauges     map[string]metric.Float64Gauge
}

// New 创建 OTLP 指标 Provider。
func New(options ...Option) (*Provider, error) {
	config := newConfig(options...)

	ctx := context.Background()
	exporter, err := newExporter(ctx, config)
	if err != nil {
		return nil, err
	}
	var res *resource.Resource
	res, err = resource.New(
		ctx,
		resource.WithAttributes(
			semconv.ServiceName(config.serviceName),
			semconv.ServiceVersion(config.serviceVersion),
		),
	)
	if err != nil {
		closeErr := exporter.Shutdown(ctx)
		if closeErr != nil {
			return nil, errors.Join(fmt.Errorf("create OpenTelemetry resource: %w", err), closeErr)
		}
		return nil, fmt.Errorf("create OpenTelemetry resource: %w", err)
	}
	reader := sdkmetric.NewPeriodicReader(
		exporter,
		sdkmetric.WithInterval(config.exportInterval),
	)
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(reader),
	)
	return &Provider{
		meter:         meterProvider.Meter(config.serviceName),
		meterProvider: meterProvider,
		counters:      make(map[string]metric.Int64Counter),
		histograms:    make(map[string]metric.Float64Histogram),
		gauges:        make(map[string]metric.Float64Gauge),
	}, nil
}

// Counter 实现 metrics.Metrics。
func (p *Provider) Counter(ctx context.Context, name string, value int64, labels map[string]string) {
	p.mu.Lock()
	counter, ok := p.counters[name]
	if !ok {
		var err error
		counter, err = p.meter.Int64Counter(name)
		if err != nil {
			p.mu.Unlock()
			return
		}
		p.counters[name] = counter
	}
	p.mu.Unlock()
	counter.Add(ctx, value, metric.WithAttributes(attributes(labels)...))
}

// Histogram 实现 metrics.Metrics。
func (p *Provider) Histogram(ctx context.Context, name string, value float64, labels map[string]string) {
	p.mu.Lock()
	histogram, ok := p.histograms[name]
	if !ok {
		var err error
		histogram, err = p.meter.Float64Histogram(name)
		if err != nil {
			p.mu.Unlock()
			return
		}
		p.histograms[name] = histogram
	}
	p.mu.Unlock()
	histogram.Record(ctx, value, metric.WithAttributes(attributes(labels)...))
}

// Gauge 实现 metrics.Metrics。
func (p *Provider) Gauge(ctx context.Context, name string, value float64, labels map[string]string) {
	p.mu.Lock()
	gauge, ok := p.gauges[name]
	if !ok {
		var err error
		gauge, err = p.meter.Float64Gauge(name)
		if err != nil {
			p.mu.Unlock()
			return
		}
		p.gauges[name] = gauge
	}
	p.mu.Unlock()
	gauge.Record(ctx, value, metric.WithAttributes(attributes(labels)...))
}

// Close 刷新并关闭 MeterProvider。
func (p *Provider) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return p.meterProvider.Shutdown(ctx)
}

// newExporter 按配置创建 OTLP exporter。
func newExporter(ctx context.Context, config config) (sdkmetric.Exporter, error) {
	var exporter sdkmetric.Exporter
	var err error
	if config.useHTTP {
		options := []otlpmetrichttp.Option{otlpmetrichttp.WithEndpoint(config.endpoint)}
		if config.insecure {
			options = append(options, otlpmetrichttp.WithInsecure())
		}
		exporter, err = otlpmetrichttp.New(ctx, options...)
	} else {
		options := []otlpmetricgrpc.Option{otlpmetricgrpc.WithEndpoint(config.endpoint)}
		if config.insecure {
			options = append(options, otlpmetricgrpc.WithInsecure())
		}
		exporter, err = otlpmetricgrpc.New(ctx, options...)
	}
	if err != nil {
		return nil, fmt.Errorf("create OTLP metric exporter: %w", err)
	}
	return exporter, nil
}

// attributes 按标签名排序生成 OpenTelemetry attributes。
func attributes(labels map[string]string) []attribute.KeyValue {
	names := make([]string, 0, len(labels))
	for name := range labels {
		names = append(names, name)
	}
	slices.Sort(names)
	values := make([]attribute.KeyValue, 0, len(names))
	for _, name := range names {
		values = append(values, attribute.String(name, labels[name]))
	}
	return values
}

// newConfig 应用选项并按传输协议补充默认端点。
func newConfig(options ...Option) config {
	cfg := config{
		serviceName:    "kratos-service",
		serviceVersion: "v0.0.1",
		exportInterval: time.Minute,
	}
	for _, option := range options {
		option(&cfg)
	}
	if cfg.endpoint == "" {
		cfg.endpoint = defaultGRPCEndpoint
		if cfg.useHTTP {
			cfg.endpoint = defaultHTTPEndpoint
		}
	}
	return cfg
}
