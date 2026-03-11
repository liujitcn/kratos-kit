package requestid

import (
	"context"

	"github.com/liujitcn/go-utils/id"

	"github.com/go-kratos/kratos/v2/middleware"
)

const DefaultRequestIDHeader = "X-Request-Id"

// RequestIDOption configures middleware behavior
type RequestIDOption func(*requestIDOptions)

type requestIDOptions struct {
	headerName string
	generator  func() string
}

// WithRequestIDHeader sets a custom header name for Request ID
func WithRequestIDHeader(name string) RequestIDOption {
	return func(o *requestIDOptions) { o.headerName = name }
}

// WithRequestIDGenerator sets a custom ID generator
func WithRequestIDGenerator(f func() string) RequestIDOption {
	return func(o *requestIDOptions) { o.generator = f }
}

// context key for request id
type ctxKeyRequestID struct{}

// GetRequestID returns the request id stored in context, or empty if none
func GetRequestID(ctx context.Context) string {
	if v := ctx.Value(ctxKeyRequestID{}); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// NewRequestIDMiddleware returns a middleware that ensures every request has a request id.
// It will read from context (if present), otherwise generate one, and store it in context so
// downstream middlewares/handlers can use GetRequestID(ctx).
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
				// already present
				return next(ctx, req)
			}
			ctx = context.WithValue(ctx, ctxKeyRequestID{}, cfg.generator())
			return next(ctx, req)
		}
	}
}
