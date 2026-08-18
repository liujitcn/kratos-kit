package requestid

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/transport"
)

// DefaultHeaderName 是默认的 HTTP 请求标识请求头。
const DefaultHeaderName = "X-Request-Id"

// HandlerMiddleware 定义 HTTP 请求标识中间件类型。
type HandlerMiddleware func(http.Handler) http.Handler

// Option 配置 HTTP 请求标识中间件。
type Option func(*options)

type options struct {
	headerName  string
	idGenerator func() string
}

// WithHeaderName 设置提取和写入请求标识的 HTTP 请求头。
func WithHeaderName(name string) Option {
	return func(o *options) {
		if name != "" {
			o.headerName = name
		}
	}
}

// WithIDGenerator 设置自定义请求标识生成函数。
func WithIDGenerator(fn func() string) Option {
	return func(o *options) {
		if fn != nil {
			o.idGenerator = fn
		}
	}
}

// Middleware 创建提取或生成请求标识的 HTTP 中间件，并将标识写回响应头。
func Middleware(opts ...Option) HandlerMiddleware {
	cfg := newOptions(opts)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get(cfg.headerName)
			if requestID == "" {
				requestID = cfg.idGenerator()
			}
			w.Header().Set(cfg.headerName, requestID)
			next.ServeHTTP(w, r.WithContext(WithRequestID(r.Context(), requestID)))
		})
	}
}

// Server 创建适用于 Kratos HTTP Server 的请求标识中间件。
func Server(opts ...Option) middleware.Middleware {
	cfg := newOptions(opts)
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			requestID := FromContext(ctx)
			transport, ok := transport.FromServerContext(ctx)
			if requestID == "" && ok {
				requestID = transport.RequestHeader().Get(cfg.headerName)
			}
			if requestID == "" {
				requestID = cfg.idGenerator()
			}
			if ok {
				transport.ReplyHeader().Set(cfg.headerName, requestID)
			}
			return next(WithRequestID(ctx, requestID), req)
		}
	}
}

// WithRequestID 将请求标识写入上下文。
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, requestID)
}

// FromContext 从上下文读取请求标识，不存在时返回空字符串。
func FromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	requestID, _ := ctx.Value(requestIDKey{}).(string)
	return requestID
}

type requestIDKey struct{}

func newOptions(opts []Option) *options {
	cfg := &options{headerName: DefaultHeaderName, idGenerator: defaultIDGenerator}
	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}
	return cfg
}

func defaultIDGenerator() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
