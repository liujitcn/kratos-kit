// Package requestid 提供跨 HTTP 和 gRPC 的请求标识生成与透传中间件。
package requestid

import (
	"context"

	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/transport"
	"github.com/liujitcn/go-utils/id"
)

// DefaultRequestIDHeader 是默认的请求标识请求头。
const DefaultRequestIDHeader = "X-Request-Id"

// RequestIDOption 配置请求标识中间件行为。
type RequestIDOption func(*requestIDOptions)

type requestIDOptions struct {
	headerName string
	generator  func() string
}

// WithRequestIDHeader 设置自定义请求标识头名称。
func WithRequestIDHeader(name string) RequestIDOption {
	return func(options *requestIDOptions) {
		options.headerName = name
	}
}

// WithRequestIDGenerator 设置自定义请求标识生成函数。
func WithRequestIDGenerator(generator func() string) RequestIDOption {
	return func(options *requestIDOptions) {
		options.generator = generator
	}
}

type requestIDKey struct{}

// WithRequestID 将请求标识写入上下文。
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, requestID)
}

// FromContext 从上下文读取请求标识，不存在时返回空字符串。
func FromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDKey{}).(string)
	return requestID
}

// GetRequestID 从上下文读取请求标识。
func GetRequestID(ctx context.Context) string {
	return FromContext(ctx)
}

// Server 创建服务端请求标识中间件。
func Server(opts ...RequestIDOption) middleware.Middleware {
	options := newOptions(opts)
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			requestID := FromContext(ctx)
			serverTransport, ok := transport.FromServerContext(ctx)
			if requestID == "" && ok {
				requestID = serverTransport.RequestHeader().Get(options.headerName)
			}
			if requestID == "" {
				requestID = options.generator()
			}
			if ok {
				serverTransport.ReplyHeader().Set(options.headerName, requestID)
			}
			return next(WithRequestID(ctx, requestID), req)
		}
	}
}

// Client 创建客户端请求标识中间件。
func Client(opts ...RequestIDOption) middleware.Middleware {
	options := newOptions(opts)
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			requestID := FromContext(ctx)
			if requestID == "" {
				requestID = options.generator()
				ctx = WithRequestID(ctx, requestID)
			}
			clientTransport, ok := transport.FromClientContext(ctx)
			if ok {
				clientTransport.RequestHeader().Set(options.headerName, requestID)
			}
			return next(ctx, req)
		}
	}
}

// NewRequestIDMiddleware 创建兼容旧调用方式的服务端请求标识中间件。
func NewRequestIDMiddleware(opts ...RequestIDOption) middleware.Middleware {
	return Server(opts...)
}

// newOptions 创建请求标识中间件配置。
func newOptions(opts []RequestIDOption) *requestIDOptions {
	options := &requestIDOptions{
		headerName: DefaultRequestIDHeader,
		generator:  id.NewGUIDv7,
	}
	for _, option := range opts {
		option(options)
	}
	return options
}
