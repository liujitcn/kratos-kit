package auth

import (
	"context"
	"regexp"
	"strings"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/liujitcn/kratos-kit/api/gen/go/conf"
	authnEngine "github.com/liujitcn/kratos-kit/auth/authn/engine"
	authnMiddleware "github.com/liujitcn/kratos-kit/auth/authn/middleware"
	authzEngine "github.com/liujitcn/kratos-kit/auth/authz/engine"
	authzMiddleware "github.com/liujitcn/kratos-kit/auth/authz/middleware"
	"github.com/liujitcn/kratos-kit/auth/data"
)

// NewAuthMiddleware 创建统一鉴权中间件，并按白名单规则决定鉴权链路。
func NewAuthMiddleware(authenticator authnEngine.Authenticator,
	authorizer authzEngine.Engine,
	userToken *data.UserToken, cfg *conf.Authentication_Jwt) middleware.Middleware {
	fullAuth := middleware.Chain(
		authnMiddleware.Server(authenticator),
		Server(userToken),
		authzMiddleware.Server(authorizer),
	)

	optionalAuth := OptionalServer(authenticator, userToken)
	return func(handler middleware.Handler) middleware.Handler {
		fullAuthHandler := fullAuth(handler)
		optionalAuthHandler := optionalAuth(handler)
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			serverTransport, ok := transport.FromServerContext(ctx)
			if !ok {
				// 无法识别请求元信息时，回退到完整鉴权链路。
				return fullAuthHandler(ctx, req)
			}

			operation := serverTransport.Operation()
			if matchWhiteList(cfg.GetOptionalAuth(), operation) {
				// 可选鉴权接口只解析身份，不强制拦截未登录请求。
				return optionalAuthHandler(ctx, req)
			}
			if matchWhiteList(cfg.GetWhiteList(), operation) {
				// 白名单接口直接透传给业务处理器。
				return handler(ctx, req)
			}
			return fullAuthHandler(ctx, req)
		}
	}
}

// OptionalServer 为白名单接口补充可选认证解析。
func OptionalServer(authenticator authnEngine.Authenticator, userToken *data.UserToken) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			authnClaims, err := authenticator.Authenticate(ctx, authnEngine.ContextTypeKratosMetaData)
			if err != nil || authnClaims == nil {
				return handler(ctx, req)
			}
			if err = verifyAccessToken(userToken, authnClaims); err != nil {
				return handler(ctx, req)
			}
			return handler(authnMiddleware.NewContext(ctx, authnClaims), req)
		}
	}
}

func matchWhiteList(whiteList *conf.Authentication_Jwt_WhiteList, operation string) bool {
	if whiteList == nil {
		return false
	}
	for _, prefix := range whiteList.Prefix {
		if strings.HasPrefix(operation, prefix) {
			return true
		}
	}
	for _, regexValue := range whiteList.Regex {
		regex, err := regexp.Compile(regexValue)
		if err != nil {
			continue
		}
		if regex.FindString(operation) == operation {
			return true
		}
	}
	for _, path := range whiteList.Path {
		if path == operation {
			return true
		}
	}
	for _, item := range whiteList.Match {
		if item == operation {
			return true
		}
	}
	return false
}
