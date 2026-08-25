package tracer

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
)

var (
	// tpInstance holds the currently active global tracer provider (interface type, nil-able)
	tpMu       sync.Mutex
	tpInstance *trace.TracerProvider
)

// NewTracerExporter 构建 exporter：优先使用注册表中的 factory。
// exporterName 不能为空，cfg 传入给 factory 用于读取 endpoint/insecure/headers 等信息。
func NewTracerExporter(ctx context.Context, cfg *configv1.Tracer) (trace.SpanExporter, error) {
	if cfg == nil {
		return nil, errors.New("tracer: tracer cfg is nil")
	}
	if cfg.GetExporter() == "" {
		return nil, errors.New("tracer: exporter name is empty")
	}

	if f, ok := GetExporterFactory(cfg.GetExporter()); ok {
		return f(ctx, cfg)
	}
	return nil, fmt.Errorf("tracer: unknown exporter %q; available: %v", cfg.GetExporter(), ListExporterNames())
}

// ShutdownTracerProvider gracefully shuts down the active global tracer provider (if set).
// Safe to call multiple times; returns error if shutdown fails.
func ShutdownTracerProvider(ctx context.Context) error {
	tpMu.Lock()
	defer tpMu.Unlock()
	if tpInstance == nil {
		return nil
	}
	if err := tpInstance.Shutdown(ctx); err != nil {
		return err
	}
	tpInstance = nil
	return nil
}

// NewTracerProvider 创建 tracer provider 并设置为全局 provider。
// 注：为了更好的资源管理，可以使用 NewTracerProviderWithShutdown 获得 shutdown 函数。
func NewTracerProvider(ctx context.Context, cfg *configv1.Tracer, appInfo *configv1.AppInfo) error {
	_, _, err := NewTracerProviderWithShutdown(ctx, cfg, appInfo)
	return err
}

// NewTracerProviderWithShutdown 返回 (tp, shutdownFunc, error)，推荐在 main 中使用并在退出时调用 shutdownFunc(ctx)
func NewTracerProviderWithShutdown(ctx context.Context, cfg *configv1.Tracer, appInfo *configv1.AppInfo) (*trace.TracerProvider, func(context.Context) error, error) {
	if cfg == nil || appInfo == nil {
		return nil, func(context.Context) error { return nil }, nil
	}

	// do not mutate caller cfg; use local defaults
	sampler := cfg.GetSampler()
	if sampler == 0 {
		sampler = 1.0
	}
	env := cfg.GetEnv()
	if env == "" {
		env = "dev"
	}

	opts := []trace.TracerProviderOption{
		trace.WithSampler(trace.ParentBased(trace.TraceIDRatioBased(sampler))),
		trace.WithResource(resource.NewSchemaless(
			semconv.ServiceNamespaceKey.String(appInfo.GetProject()),
			semconv.ServiceNameKey.String(appInfo.GetAppId()),
			semconv.ServiceVersionKey.String(appInfo.GetVersion()),
			semconv.ServiceInstanceIDKey.String(appInfo.GetInstanceId()),
			attribute.String("service.env", env),
		)),
	}

	// NOTE: cfg.GetBatcher() historically used as exporter name in this project.
	// Consider renaming configv1.Tracer fields to `Exporter`/`ExporterName` in the future.
	if len(cfg.GetEndpoint()) > 0 {
		exp, err := NewTracerExporter(ctx, cfg)
		if err != nil {
			return nil, nil, err
		}
		opts = append(opts, trace.WithBatcher(exp))
	}

	tp := trace.NewTracerProvider(opts...)

	// defensive check (NewTracerProvider does not return nil in normal cases)
	if tp == nil {
		return nil, nil, errors.New("tracer: create tracer provider failed")
	}

	// set global provider and keep reference for shutdown
	tpMu.Lock()
	// shutdown previous provider if present
	if tpInstance != nil {
		_ = tpInstance.Shutdown(ctx)
	}
	tpInstance = tp
	tpMu.Unlock()

	// set global tracer provider
	otel.SetTracerProvider(tp)

	// set global propagator
	otel.SetTextMapPropagator(
		NewCompositePropagator(
			cfg.GetEnableTraceContext(),
			cfg.GetEnableBaggage(),
		),
	)

	shutdown := func(c context.Context) error {
		// shutdown global provider if it's still the same one
		return ShutdownTracerProvider(c)
	}

	return tp, shutdown, nil
}

// NewCompositePropagator 构建一个复合 propagator。
// - enableTraceContext: 是否包含 W3C TraceContext
// - enableBaggage: 是否包含 Baggage
// 返回值保证非 nil，默认回退为 TraceContext。
func NewCompositePropagator(enableTraceContext, enableBaggage bool) propagation.TextMapPropagator {
	var parts []propagation.TextMapPropagator
	if enableTraceContext {
		parts = append(parts, propagation.TraceContext{})
	}
	if enableBaggage {
		parts = append(parts, propagation.Baggage{})
	}

	switch len(parts) {
	case 0:
		return propagation.TraceContext{}
	case 1:
		return parts[0]
	default:
		return propagation.NewCompositeTextMapPropagator(parts...)
	}
}
