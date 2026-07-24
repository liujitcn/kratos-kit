package requestid

import (
	"context"

	"github.com/liujitcn/go-utils/id"

	"github.com/go-kratos/kratos/v3/middleware"
)

const DefaultRequestIDHeader = "X-Request-Id"

// RequestIDOption 配置 Request ID 中间件行为。
type RequestIDOption func(*requestIDOptions)

type requestIDOptions struct {
	headerName string
	generator  func() string
}

// WithRequestIDHeader 设置自定义 Request ID 头名称。
func WithRequestIDHeader(name string) RequestIDOption {
	return func(o *requestIDOptions) { o.headerName = name }
}

// WithRequestIDGenerator 设置自定义 Request ID 生成函数。
func WithRequestIDGenerator(f func() string) RequestIDOption {
	return func(o *requestIDOptions) { o.generator = f }
}

// ctxKeyRequestID 是上下文中的 Request ID 键。
type ctxKeyRequestID struct{}

// GetRequestID 从上下文中读取 Request ID，不存在时返回空字符串。
func GetRequestID(ctx context.Context) string {
	if v := ctx.Value(ctxKeyRequestID{}); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// NewRequestIDMiddleware 创建确保每个请求都带有 Request ID 的中间件。
// 若上下文中已存在 Request ID，会继续复用；否则生成新 ID 并写入上下文，供后续中间件和处理器读取。
func NewRequestIDMiddleware(opts ...RequestIDOption) middleware.Middleware {
	cfg := &requestIDOptions{
		headerName: DefaultRequestIDHeader,
		generator: func() string {
			return id.NewGUIDv7()
		},
	}
	for _, o := range opts {
		o(cfg)
	}

	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			if requestID := GetRequestID(ctx); requestID != "" {
				// 已存在 Request ID 时不再重复生成，避免覆盖上游传入的链路标识。
				return next(ctx, req)
			}
			ctx = context.WithValue(ctx, ctxKeyRequestID{}, cfg.generator())
			return next(ctx, req)
		}
	}
}
