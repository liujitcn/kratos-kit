package metrics

import (
	"context"
	"strings"
	"time"

	"github.com/liujitcn/kratos-kit/metrics"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	defaultRequestCounter   = "grpc_requests_total"
	defaultLatencyHistogram = "grpc_request_duration_seconds"
	defaultInFlightGauge    = "grpc_requests_in_flight"
)

// Option 配置 gRPC 指标拦截器。
type Option func(*options)

type options struct {
	requestCounter   string
	latencyHistogram string
	inFlightGauge    string
	labelFunc        func(method string, err error) map[string]string
	skipFunc         func(method string) bool
}

// WithRequestCounterName 设置请求计数器名称。
func WithRequestCounterName(name string) Option {
	return func(o *options) { o.requestCounter = name }
}

// WithLatencyHistogramName 设置耗时直方图名称。
func WithLatencyHistogramName(name string) Option {
	return func(o *options) { o.latencyHistogram = name }
}

// WithInFlightGaugeName 设置进行中请求仪表盘名称。
func WithInFlightGaugeName(name string) Option {
	return func(o *options) { o.inFlightGauge = name }
}

// WithLabelFunc 设置自定义指标标签函数。
func WithLabelFunc(fn func(method string, err error) map[string]string) Option {
	return func(o *options) {
		if fn != nil {
			o.labelFunc = fn
		}
	}
}

// WithSkipFunc 设置跳过指标记录的方法判断函数。
func WithSkipFunc(fn func(method string) bool) Option {
	return func(o *options) { o.skipFunc = fn }
}

// UnaryServerInterceptor 创建服务端一元 RPC 指标拦截器。
func UnaryServerInterceptor(m metrics.Metrics, opts ...Option) grpc.UnaryServerInterceptor {
	cfg := newOptions(opts)
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if cfg.skipFunc != nil && cfg.skipFunc(info.FullMethod) {
			return handler(ctx, req)
		}
		labels := cfg.labelFunc(info.FullMethod, nil)
		m.Gauge(ctx, cfg.inFlightGauge, 1, labels)
		defer m.Gauge(ctx, cfg.inFlightGauge, 0, labels)
		start := time.Now()
		resp, err := handler(ctx, req)
		finish(m, cfg, ctx, info.FullMethod, start, err)
		return resp, err
	}
}

// StreamServerInterceptor 创建服务端流式 RPC 指标拦截器。
func StreamServerInterceptor(m metrics.Metrics, opts ...Option) grpc.StreamServerInterceptor {
	cfg := newOptions(opts)
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if cfg.skipFunc != nil && cfg.skipFunc(info.FullMethod) {
			return handler(srv, stream)
		}
		labels := cfg.labelFunc(info.FullMethod, nil)
		m.Gauge(stream.Context(), cfg.inFlightGauge, 1, labels)
		defer m.Gauge(stream.Context(), cfg.inFlightGauge, 0, labels)
		start := time.Now()
		err := handler(srv, stream)
		finish(m, cfg, stream.Context(), info.FullMethod, start, err)
		return err
	}
}

func finish(m metrics.Metrics, cfg *options, ctx context.Context, method string, start time.Time, err error) {
	labels := cfg.labelFunc(method, err)
	m.Counter(ctx, cfg.requestCounter, 1, labels)
	m.Histogram(ctx, cfg.latencyHistogram, time.Since(start).Seconds(), labels)
}

func newOptions(opts []Option) *options {
	cfg := &options{
		requestCounter:   defaultRequestCounter,
		latencyHistogram: defaultLatencyHistogram,
		inFlightGauge:    defaultInFlightGauge,
		labelFunc:        defaultLabels,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}
	return cfg
}

func defaultLabels(method string, err error) map[string]string {
	code := codes.OK.String()
	if err != nil {
		st, _ := status.FromError(err)
		code = st.Code().String()
	}
	return map[string]string{"method": method, "service": serviceFromMethod(method), "code": code}
}

func serviceFromMethod(fullMethod string) string {
	parts := strings.Split(fullMethod, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2]
	}
	return fullMethod
}
