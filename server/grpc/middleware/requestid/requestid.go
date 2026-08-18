package requestid

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/transport"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// MetadataKey 是 gRPC 请求标识使用的 metadata 键。
// gRPC 底层传输会将 metadata 键规范化为小写，但对外保持该名称不变。
const MetadataKey = "X-Request-Id"

// Option 配置 gRPC 请求标识拦截器。
type Option func(*options)

type options struct {
	idGenerator func() string
}

// WithIDGenerator 设置自定义请求标识生成函数。
func WithIDGenerator(fn func() string) Option {
	return func(o *options) {
		if fn != nil {
			o.idGenerator = fn
		}
	}
}

// UnaryServerInterceptor 创建提取 metadata 请求标识并写入上下文的一元服务端拦截器。
func UnaryServerInterceptor(opts ...Option) grpc.UnaryServerInterceptor {
	cfg := newOptions(opts)
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		requestID := extractFromIncoming(ctx)
		if requestID == "" {
			requestID = cfg.idGenerator()
		}
		return handler(WithRequestID(ctx, requestID), req)
	}
}

// StreamServerInterceptor 创建提取 metadata 请求标识的流式服务端拦截器。
func StreamServerInterceptor(opts ...Option) grpc.StreamServerInterceptor {
	cfg := newOptions(opts)
	return func(srv any, stream grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		requestID := extractFromIncoming(stream.Context())
		if requestID == "" {
			requestID = cfg.idGenerator()
		}
		wrapped := &wrappedServerStream{ServerStream: stream, ctx: WithRequestID(stream.Context(), requestID)}
		return handler(srv, wrapped)
	}
}

// Server 创建适用于 Kratos gRPC Server 的请求标识中间件。
func Server(opts ...Option) middleware.Middleware {
	cfg := newOptions(opts)
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			requestID := FromContext(ctx)
			transport, ok := transport.FromServerContext(ctx)
			if requestID == "" && ok {
				requestID = transport.RequestHeader().Get(MetadataKey)
			}
			if requestID == "" {
				requestID = cfg.idGenerator()
			}
			if ok {
				transport.ReplyHeader().Set(MetadataKey, requestID)
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
	cfg := &options{idGenerator: defaultIDGenerator}
	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}
	return cfg
}

func extractFromIncoming(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	values := md.Get(MetadataKey)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func defaultIDGenerator() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *wrappedServerStream) Context() context.Context {
	return s.ctx
}
