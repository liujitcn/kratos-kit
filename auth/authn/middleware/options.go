package middleware

import (
	"github.com/liujitcn/kratos-kit/auth/authn/engine"
)

type Option func(*options)

type options struct {
	claims engine.AuthClaims
}

// WithAuthClaims 设置客户端中间件默认注入的认证声明。
func WithAuthClaims(claims engine.AuthClaims) Option {
	return func(o *options) {
		o.claims = claims
	}
}
