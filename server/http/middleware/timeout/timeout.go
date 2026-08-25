package timeout

import (
	"bytes"
	"context"
	"net/http"
	"slices"
	"sync"
	"time"
)

// Option 配置超时中间件。
type Option func(*options)

type options struct {
	status      int
	message     string
	skipFunc    func(r *http.Request) bool
	timeoutFunc func(r *http.Request) time.Duration
}

// WithStatus 设置超时响应状态码。
func WithStatus(code int) Option { return func(o *options) { o.status = code } }

// WithMessage 设置超时响应正文。
func WithMessage(message string) Option { return func(o *options) { o.message = message } }

// WithSkipFunc 设置跳过超时的请求判断函数。
func WithSkipFunc(fn func(r *http.Request) bool) Option { return func(o *options) { o.skipFunc = fn } }

// WithTimeoutFunc 设置按请求计算超时时间的函数。
func WithTimeoutFunc(fn func(r *http.Request) time.Duration) Option {
	return func(o *options) { o.timeoutFunc = fn }
}

// Middleware 创建限制下游处理器执行时间的 HTTP 中间件。
func Middleware(defaultTimeout time.Duration, opts ...Option) func(http.Handler) http.Handler {
	cfg := &options{status: http.StatusServiceUnavailable, message: "Service Unavailable"}
	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cfg.skipFunc != nil && cfg.skipFunc(r) {
				next.ServeHTTP(w, r)
				return
			}
			timeout := defaultTimeout
			if cfg.timeoutFunc != nil {
				if value := cfg.timeoutFunc(r); value > 0 {
					timeout = value
				}
			}
			if timeout <= 0 {
				next.ServeHTTP(w, r)
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			buffered := newBufferedResponseWriter(w)
			done := make(chan any, 1)
			go func() {
				var panicValue any
				func() {
					defer func() { panicValue = recover() }()
					next.ServeHTTP(buffered, r.WithContext(ctx))
				}()
				done <- panicValue
			}()
			select {
			case panicValue := <-done:
				if panicValue != nil {
					panic(panicValue)
				}
				buffered.flushTo(w)
			case <-ctx.Done():
				http.Error(w, cfg.message, cfg.status)
			}
		})
	}
}

type bufferedResponseWriter struct {
	mu          sync.Mutex
	header      http.Header
	status      int
	wroteHeader bool
	body        bytes.Buffer
}

func newBufferedResponseWriter(w http.ResponseWriter) *bufferedResponseWriter {
	header := make(http.Header, len(w.Header()))
	for key, values := range w.Header() {
		header[key] = slices.Clone(values)
	}
	return &bufferedResponseWriter{header: header, status: http.StatusOK}
}

// Header 返回缓冲响应头。
func (w *bufferedResponseWriter) Header() http.Header { return w.header }

// WriteHeader 记录响应状态码。
func (w *bufferedResponseWriter) WriteHeader(status int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.wroteHeader {
		return
	}
	w.status = status
	w.wroteHeader = true
}

// Write 缓冲响应正文并处理隐式 200 状态。
func (w *bufferedResponseWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.wroteHeader {
		w.wroteHeader = true
	}
	return w.body.Write(data)
}

// Flush 满足流式处理器的接口要求；响应会在处理器结束后统一写出。
func (w *bufferedResponseWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.wroteHeader = true
}

func (w *bufferedResponseWriter) flushTo(dst http.ResponseWriter) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for key, values := range w.header {
		dst.Header()[key] = slices.Clone(values)
	}
	dst.WriteHeader(w.status)
	_, _ = dst.Write(w.body.Bytes())
}
