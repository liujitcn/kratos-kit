package tracing

import (
	"context"

	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/transport"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Server 创建服务端链路追踪中间件。
func Server(opts ...Option) middleware.Middleware {
	tracer := NewTracer(trace.SpanKindServer, "", opts...)
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			tr, ok := transport.FromServerContext(ctx)
			if !ok {
				return handler(ctx, req)
			}

			ctx, span := tracer.StartWithName(ctx, tr.Operation(), tr.RequestHeader(), transportAttrs(tr)...)
			reply, err := handler(ctx, req)
			tracer.End(ctx, span, err)
			return reply, err
		}
	}
}

// Client 创建客户端链路追踪中间件。
func Client(opts ...Option) middleware.Middleware {
	tracer := NewTracer(trace.SpanKindClient, "", opts...)
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			tr, ok := transport.FromClientContext(ctx)
			if !ok {
				return handler(ctx, req)
			}

			ctx, span := tracer.StartWithName(ctx, tr.Operation(), tr.RequestHeader(), transportAttrs(tr)...)
			reply, err := handler(ctx, req)
			tracer.End(ctx, span, err)
			return reply, err
		}
	}
}

// transportAttrs 将 Kratos transport 信息转换为 span 属性。
func transportAttrs(tr transport.Transporter) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("rpc.system", tr.Kind().String()),
		attribute.String("rpc.method", tr.Operation()),
		attribute.String("net.peer.name", tr.Endpoint()),
	}
}
