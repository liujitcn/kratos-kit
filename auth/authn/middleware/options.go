package middleware

import (
	"github.com/liujitcn/kratos-kit/auth/authn/engine"
)

type Option func(*options)

type options struct {
	claims          engine.AuthClaims
	authErrorMapper func(error) error
}

// WithAuthClaims 设置客户端中间件默认注入的认证声明。
func WithAuthClaims(claims engine.AuthClaims) Option {
	return func(o *options) {
		o.claims = claims
	}
}

// WithAuthErrorMapper 设置服务端认证失败时的业务错误映射。
func WithAuthErrorMapper(authErrorMapper func(error) error) Option {
	return func(o *options) {
		o.authErrorMapper = authErrorMapper
	}
}
