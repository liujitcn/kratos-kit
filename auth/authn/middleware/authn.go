package middleware

import (
	"context"

	"github.com/go-kratos/kratos/v3/log"
	"github.com/go-kratos/kratos/v3/middleware"

	"github.com/liujitcn/kratos-kit/auth/authn/engine"
)

// Server 创建服务端认证中间件。
func Server(authenticator engine.RequestAuthenticator, opts ...Option) middleware.Middleware {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			claims, err := authenticator.Authenticate(ctx, engine.ContextTypeKratosMetaData, req)
			if err != nil {
				// 认证失败时统一返回未授权错误，避免把底层实现细节直接暴露给调用方。
				log.Error("authn.middleware: authenticator middleware authenticate failed", "error", err)
				if o.authErrorMapper != nil {
					// 上层可按业务语义将缺失、过期等底层错误映射为稳定错误。
					return nil, o.authErrorMapper(err)
				}
				return nil, ErrUnauthorized
			}

			ctx = engine.ContextWithAuthClaims(ctx, claims)

			return handler(ctx, req)
		}
	}
}

// Client 创建客户端认证中间件。
func Client(authenticator engine.ContextIdentityCreator, opts ...Option) middleware.Middleware {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			var err error
			if ctx, err = authenticator.CreateIdentityWithContext(ctx, engine.ContextTypeKratosMetaData, o.claims, req); err != nil {
				// 客户端令牌创建失败仅记录日志，保留原调用链继续执行，由下游自行决定是否拦截。
				log.Error("authn.middleware: authenticator middleware create token failed", "error", err)
			}
			return handler(ctx, req)
		}
	}
}
