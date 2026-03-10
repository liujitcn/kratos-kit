package middleware

import (
	"context"

	"github.com/liujitcn/kratos-kit/auth/authz/engine"
)

func NewContext(ctx context.Context, claims *engine.AuthClaims) context.Context {
	return engine.ContextWithAuthClaims(ctx, claims)
}

func FromContext(ctx context.Context) (*engine.AuthClaims, bool) {
	return engine.AuthClaimsFromContext(ctx)
}
