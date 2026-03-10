package auth

import (
	"context"

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
	jwtBuilder := selector.Server(
		authnMiddleware.Server(authenticator),
		Server(userToken),
		authzMiddleware.Server(authorizer),
	)
	operationMap := make(map[string]struct{})
	if cfg != nil {
		whiteList := cfg.WhiteList
		if whiteList != nil {
			prefix := whiteList.Prefix
			if len(prefix) != 0 {
				jwtBuilder.Prefix(prefix...)
			}
			regex := whiteList.Regex
			if len(regex) != 0 {
				jwtBuilder.Regex(regex...)
			}
			path := whiteList.Path
			if len(path) != 0 {
				jwtBuilder.Path(path...)
			}
			match := whiteList.Match
			if len(match) != 0 {
				for _, item := range match {
					operationMap[item] = struct{}{}
				}
			}
		}
	}
	jwtBuilder.Match(func(ctx context.Context, operation string) bool {
		if _, ok := operationMap[operation]; ok {
			return false
		}
		return true
	})
	return jwtBuilder.Build()
}
