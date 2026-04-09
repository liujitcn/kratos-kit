package middleware

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"

	"github.com/liujitcn/kratos-kit/auth/authz/engine"
)

// Server 创建服务端鉴权中间件。
func Server(authorizer engine.Authorizer, opts ...Option) middleware.Middleware {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	if authorizer == nil {
		return nil
	}

	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			var (
				allowed bool
				err     error
			)

			claims, ok := engine.AuthClaimsFromContext(ctx)
			if !ok {
				// 缺少认证声明时无法继续鉴权，直接返回标准错误。
				log.Error("authz middleware: missing auth claims in context")
				return nil, ErrMissingClaims
			}

			if claims.Action == nil || claims.Resource == nil {
				log.Error("authz middleware: missing auth claims in context")
				return nil, ErrInvalidClaims
			}

			var project engine.Project
			if claims.Project == nil {
				project = ""
			} else {
				project = *claims.Project
			}

			if claims.Subject != nil {
				allowed, err = authorizer.IsAuthorized(ctx, *claims.Subject, *claims.Action, *claims.Resource, project)
				if err != nil {
					log.Errorf("authz middleware: authorization failed for subject %s, action %s, resource %s, project %s: %v",
						*claims.Subject, *claims.Action, *claims.Resource, project, err)
					return nil, err
				}
				if !allowed {
					return nil, ErrUnauthorized
				}
			} else if claims.Subjects != nil && len(*claims.Subjects) > 0 {
				for _, subject := range *claims.Subjects {
					allowed, err = authorizer.IsAuthorized(ctx, engine.Subject(subject), *claims.Action, *claims.Resource, project)
					if err != nil {
						log.Errorf("authz middleware: authorization failed for subject %s, action %s, resource %s, project %s: %v",
							subject, *claims.Action, *claims.Resource, project, err)
						return nil, err
					}
					if allowed {
						// 只要任一主体通过鉴权即可放行，后续无需继续遍历。
						break
					}
				}
				if !allowed {
					return nil, ErrUnauthorized
				}
			} else {
				log.Error("authz middleware: missing subject in auth claims")
				return nil, ErrMissingSubject
			}

			return handler(ctx, req)
		}
	}
}
