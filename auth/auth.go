package auth

import (
	"context"
	"strings"

	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/transport"
	"github.com/liujitcn/kratos-kit/auth/authn/engine"
	authnMiddleware "github.com/liujitcn/kratos-kit/auth/authn/middleware"
	authzEngine "github.com/liujitcn/kratos-kit/auth/authz/engine"
	authzMiddleware "github.com/liujitcn/kratos-kit/auth/authz/middleware"
	"github.com/liujitcn/kratos-kit/auth/data"
)

var Action = authzEngine.Action("ANY")

// Server 衔接认证和权鉴
func Server(userToken *data.UserToken) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			tr, ok := transport.FromServerContext(ctx)
			if !ok {
				return nil, ErrWrongContext
			}

			var authnClaims *engine.AuthClaims
			authnClaims, ok = authnMiddleware.FromContext(ctx)
			if !ok {
				return nil, ErrWrongContext
			}

			// 校验访问令牌是否存在
			if err := verifyAccessToken(ctx, userToken, authnClaims); err != nil {
				return nil, err
			}

			tenantCode, err := authnClaims.GetString(data.ClaimFieldTenantCode)
			if err != nil || tenantCode == "" {
				return nil, ErrExtractTenantFailed
			}
			var roleCode string
			roleCode, err = authnClaims.GetString(data.ClaimFieldRoleCode)
			if err != nil || roleCode == "" {
				return nil, ErrExtractSubjectFailed
			}
			authzClaims := authzEngine.AuthClaims{
				Tenant:   new(authzEngine.Tenant(tenantCode)),
				Subject:  new(authzEngine.Subject(roleCode)),
				Action:   &Action,
				Resource: new(authzEngine.Resource(tr.Operation())),
			}

			ctx = authzMiddleware.NewContext(ctx, &authzClaims)

			return handler(ctx, req)
		}
	}
}

func FromContext(ctx context.Context) (*data.UserTokenPayload, error) {
	claims, ok := engine.AuthClaimsFromContext(ctx)
	if !ok {
		return nil, ErrMissingJwtToken
	}

	return data.NewUserTokenPayloadWithClaims(claims)
}

// verifyAccessToken 校验访问令牌
func verifyAccessToken(ctx context.Context, userToken *data.UserToken, authnClaims *engine.AuthClaims) error {
	userID, err := authnClaims.GetInt64(data.ClaimFieldUserID)
	if err != nil {
		return ErrExtractUserInfoFailed
	}
	// 用户id == 0 内部调用
	if userID == 0 {
		return nil
	}
	// 校验当前请求携带的访问令牌，支持同一用户保留多个会话。
	accessToken := ""
	if tr, ok := transport.FromServerContext(ctx); ok {
		parts := strings.Fields(tr.RequestHeader().Get("Authorization"))
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			accessToken = parts[1]
		}
	}
	if !userToken.IsAccessTokenValid(userID, accessToken) {
		return ErrAccessTokenExpired
	}

	return nil
}
