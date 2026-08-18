package timeout

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Option 配置超时拦截器。
type Option func(*options)

type options struct {
	skipFunc func(method string) bool
}

// WithSkipFunc 设置跳过超时的方法判断函数。
func WithSkipFunc(fn func(method string) bool) Option {
	return func(o *options) { o.skipFunc = fn }
}

// UnaryServerInterceptor 创建服务端一元 RPC 超时拦截器。
func UnaryServerInterceptor(defaultTimeout time.Duration, opts ...Option) grpc.UnaryServerInterceptor {
	cfg := newOptions(opts)
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if cfg.skipFunc != nil && cfg.skipFunc(info.FullMethod) {
			return handler(ctx, req)
		}
		if _, ok := ctx.Deadline(); !ok && defaultTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, defaultTimeout)
			defer cancel()
		}
		resp, err := handler(ctx, req)
		if err != nil {
			return resp, err
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			if ctxErr == context.Canceled {
				return nil, status.Error(codes.Canceled, "handler canceled")
			}
			return nil, status.Error(codes.DeadlineExceeded, "handler exceeded deadline")
		}
		return resp, nil
	}
}

// StreamServerInterceptor 创建服务端流式 RPC 超时拦截器。
func StreamServerInterceptor(defaultTimeout time.Duration, opts ...Option) grpc.StreamServerInterceptor {
	cfg := newOptions(opts)
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if cfg.skipFunc != nil && cfg.skipFunc(info.FullMethod) {
			return handler(srv, stream)
		}
		ctx := stream.Context()
		if _, ok := ctx.Deadline(); !ok && defaultTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, defaultTimeout)
			defer cancel()
			stream = &wrappedServerStream{ServerStream: stream, ctx: ctx}
		}
		return handler(srv, stream)
	}
}

func newOptions(opts []Option) *options {
	cfg := &options{}
	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}
	return cfg
}

type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *wrappedServerStream) Context() context.Context { return s.ctx }
