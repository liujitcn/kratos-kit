package middleware

import (
	"context"

	"github.com/go-kratos/kratos/v3/log"
	"github.com/go-kratos/kratos/v3/middleware"

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

			tenants := makeAuthTenants(claims)
			if len(tenants) == 0 {
				log.Error("authz middleware: missing tenant in auth claims")
				return nil, ErrInvalidClaims
			}

			var project engine.Project
			if claims.Project == nil {
				project = ""
			} else {
				project = *claims.Project
			}

			if claims.Subject != nil {
				allowed, err = isAuthorized(ctx, authorizer, tenants, *claims.Subject, *claims.Action, *claims.Resource, project)
				if err != nil {
					return nil, err
				}
				if !allowed {
					return nil, ErrUnauthorized
				}
			} else if claims.Subjects != nil && len(*claims.Subjects) > 0 {
				for _, subject := range *claims.Subjects {
					allowed, err = isAuthorized(ctx, authorizer, tenants, engine.Subject(subject), *claims.Action, *claims.Resource, project)
					if err != nil {
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

// makeAuthTenants 从鉴权声明中整理需要尝试匹配的租户集合。
func makeAuthTenants(claims *engine.AuthClaims) engine.Tenants {
	if claims.Tenant != nil && *claims.Tenant != "" {
		return engine.MakeTenants(*claims.Tenant)
	}
	if claims.Tenants != nil {
		tenants := make(engine.Tenants, 0, len(*claims.Tenants))
		for _, tenant := range *claims.Tenants {
			if tenant == "" {
				continue
			}
			tenants = append(tenants, engine.Tenant(tenant))
		}
		return tenants
	}
	return nil
}

// isAuthorized 判断主体在任一租户下是否具备指定资源动作权限。
func isAuthorized(ctx context.Context, authorizer engine.Authorizer, tenants engine.Tenants, subject engine.Subject, action engine.Action, resource engine.Resource, project engine.Project) (bool, error) {
	for _, tenant := range tenants {
		tenantClaims := engine.AuthClaims{
			Tenant: &tenant,
		}
		tenantCtx := engine.ContextWithAuthClaims(ctx, &tenantClaims)

		allowed, err := authorizer.IsAuthorized(tenantCtx, subject, action, resource, project)
		if err != nil {
			log.Error("authz middleware: authorization failed",
				"tenant", tenant,
				"subject", subject,
				"action", action,
				"resource", resource,
				"project", project,
				"error", err)
			return false, err
		}
		if allowed {
			// 只要任一租户通过鉴权即可放行，后续无需继续遍历。
			return true, nil
		}
	}
	return false, nil
}
