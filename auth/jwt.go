package auth

import (
	"context"
	"regexp"
	"strings"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	"github.com/liujitcn/kratos-kit/api/gen/go/conf"
	authnEngine "github.com/liujitcn/kratos-kit/auth/authn/engine"
	authnMiddleware "github.com/liujitcn/kratos-kit/auth/authn/middleware"
	authzEngine "github.com/liujitcn/kratos-kit/auth/authz/engine"
	authzMiddleware "github.com/liujitcn/kratos-kit/auth/authz/middleware"
	"github.com/liujitcn/kratos-kit/auth/data"
)

func NewAuthMiddleware(authenticator authnEngine.Authenticator,
	authorizer authzEngine.Engine,
	userToken *data.UserToken, cfg *conf.Authentication_Jwt) middleware.Middleware {
	optionalAuthBuilder := selector.Server(
		OptionalServer(authenticator, userToken),
	)
	if cfg != nil {
		optionalAuthBuilder.Match(func(ctx context.Context, operation string) bool {
			return matchWhiteList(cfg.OptionalAuth, operation)
		})
	}

	jwtBuilder := selector.Server(
		authnMiddleware.Server(authenticator),
		Server(userToken),
		authzMiddleware.Server(authorizer),
	)
	if cfg != nil {
		jwtBuilder.Match(func(ctx context.Context, operation string) bool {
			return !matchWhiteList(cfg.WhiteList, operation)
		})
	}
	return middleware.Chain(optionalAuthBuilder.Build(), jwtBuilder.Build())
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
