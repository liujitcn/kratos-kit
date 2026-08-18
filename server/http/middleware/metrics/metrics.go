package metrics

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/liujitcn/kratos-kit/metrics"
)

const (
	defaultRequestCounter   = "http_requests_total"
	defaultLatencyHistogram = "http_request_duration_seconds"
	defaultInFlightGauge    = "http_requests_in_flight"
)

// Option 配置指标中间件。
type Option func(*options)

type options struct {
	requestCounter   string
	latencyHistogram string
	inFlightGauge    string
	labelFunc        func(r *http.Request, status int) map[string]string
	skipFunc         func(r *http.Request) bool
}

// WithRequestCounterName 设置请求计数器名称。
func WithRequestCounterName(name string) Option { return func(o *options) { o.requestCounter = name } }

// WithLatencyHistogramName 设置耗时直方图名称。
func WithLatencyHistogramName(name string) Option {
	return func(o *options) { o.latencyHistogram = name }
}

// WithInFlightGaugeName 设置进行中请求仪表盘名称。
func WithInFlightGaugeName(name string) Option { return func(o *options) { o.inFlightGauge = name } }

// WithLabelFunc 设置自定义指标标签函数。
func WithLabelFunc(fn func(r *http.Request, status int) map[string]string) Option {
	return func(o *options) {
		if fn != nil {
			o.labelFunc = fn
		}
	}
}

// WithSkipFunc 设置跳过指标采集的请求判断函数。
func WithSkipFunc(fn func(r *http.Request) bool) Option { return func(o *options) { o.skipFunc = fn } }

// Middleware 创建 HTTP 指标中间件。
func Middleware(m metrics.Metrics, opts ...Option) func(http.Handler) http.Handler {
	cfg := newOptions(opts)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cfg.skipFunc != nil && cfg.skipFunc(r) {
				next.ServeHTTP(w, r)
				return
			}
			gaugeLabels := cfg.labelFunc(r, 0)
			m.Gauge(r.Context(), cfg.inFlightGauge, 1, gaugeLabels)
			defer m.Gauge(r.Context(), cfg.inFlightGauge, 0, gaugeLabels)
			recorder := acquireStatusRecorder(w)
			defer releaseStatusRecorder(recorder)
			start := time.Now()
			next.ServeHTTP(recorder, r)
			labels := cfg.labelFunc(r, recorder.status)
			m.Counter(r.Context(), cfg.requestCounter, 1, labels)
			m.Histogram(r.Context(), cfg.latencyHistogram, time.Since(start).Seconds(), labels)
		})
	}
}

func newOptions(opts []Option) *options {
	cfg := &options{requestCounter: defaultRequestCounter, latencyHistogram: defaultLatencyHistogram, inFlightGauge: defaultInFlightGauge, labelFunc: defaultLabels}
	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}
	return cfg
}

func defaultLabels(r *http.Request, status int) map[string]string {
	return map[string]string{"method": r.Method, "path": r.URL.Path, "status": strconv.Itoa(status)}
}

var statusRecorderPool = sync.Pool{New: func() any { return &statusRecorder{} }}

func acquireStatusRecorder(w http.ResponseWriter) *statusRecorder {
	recorder := statusRecorderPool.Get().(*statusRecorder)
	recorder.ResponseWriter = w
	recorder.status = http.StatusOK
	recorder.wroteHeader = false
	return recorder
}

func releaseStatusRecorder(recorder *statusRecorder) {
	recorder.ResponseWriter = nil
	statusRecorderPool.Put(recorder)
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if r.wroteHeader {
		return
	}
	r.wroteHeader = true
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(data []byte) (int, error) {
	if !r.wroteHeader {
		r.wroteHeader = true
	}
	return r.ResponseWriter.Write(data)
}

func (r *statusRecorder) Flush() {
	if !r.wroteHeader {
		r.wroteHeader = true
	}
	_ = http.NewResponseController(r.ResponseWriter).Flush()
}

func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }
