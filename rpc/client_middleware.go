package rpc

import (
	"fmt"

	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/middleware/circuitbreaker"
	"github.com/go-kratos/kratos/v3/middleware/metadata"
	"github.com/go-kratos/kratos/v3/middleware/recovery"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/liujitcn/kratos-kit/auth/authn/engine"
	"github.com/liujitcn/kratos-kit/auth/authn/engine/jwt"
	authnMiddleware "github.com/liujitcn/kratos-kit/auth/authn/middleware"
	"github.com/liujitcn/kratos-kit/rpc/middleware/requestid"
	kitTracing "github.com/liujitcn/kratos-kit/tracing"
)

// appendClientMiddlewares 按客户端配置追加 Kratos 中间件。
func appendClientMiddlewares(
	config *configv1.Client_Middleware,
	middlewares []middleware.Middleware,
) ([]middleware.Middleware, error) {
	if config == nil {
		return middlewares, nil
	}
	middlewares = append(middlewares, requestid.Client())
	if config.GetEnableRecovery() {
		middlewares = append(middlewares, recovery.Recovery())
	}
	if config.GetEnableTracing() {
		middlewares = append(middlewares, kitTracing.Client())
	}
	if config.GetEnableMetadata() {
		middlewares = append(middlewares, metadata.Client())
	}
	if config.GetEnableCircuitBreaker() {
		middlewares = append(middlewares, circuitbreaker.Client())
	}

	authConfig := config.GetAuth()
	if authConfig == nil {
		return middlewares, nil
	}
	var authenticator engine.ContextIdentityCreator
	var err error
	authenticator, err = jwt.NewAuthenticator(
		jwt.WithKey([]byte(authConfig.GetSecret())),
		jwt.WithSigningMethod(authConfig.GetMethod()),
	)
	if err != nil {
		return nil, fmt.Errorf("create JWT authenticator: %w", err)
	}
	middlewares = append(middlewares, authnMiddleware.Client(authenticator))
	return middlewares, nil
}
