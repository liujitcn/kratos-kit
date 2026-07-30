package engine

import (
	"context"
)

// RequestAuthenticator 从请求上下文认证身份。
type RequestAuthenticator interface {
	// Authenticate 认证请求并返回身份声明。
	Authenticate(requestContext context.Context, contextType ContextType, request any) (*AuthClaims, error)
}

// TokenAuthenticator 直接认证令牌字符串。
type TokenAuthenticator interface {
	// AuthenticateToken 认证令牌并返回身份声明。
	AuthenticateToken(token string) (*AuthClaims, error)
}

// ContextIdentityCreator 把身份凭证注入请求上下文。
type ContextIdentityCreator interface {
	// CreateIdentityWithContext 根据身份声明创建凭证并写入上下文。
	CreateIdentityWithContext(requestContext context.Context, contextType ContextType, claims AuthClaims, request any) (context.Context, error)
}

// IdentityCreator 根据身份声明创建令牌。
type IdentityCreator interface {
	// CreateIdentity 创建可传输的身份令牌。
	CreateIdentity(claims AuthClaims) (string, error)
}

// AuthenticatorCloser 释放认证器持有的资源。
type AuthenticatorCloser interface {
	// Close 停止认证器后台任务并释放资源。
	Close()
}

// Authenticator 是兼容旧调用方的完整认证能力组合。
type Authenticator interface {
	RequestAuthenticator
	TokenAuthenticator
	ContextIdentityCreator
	IdentityCreator
	AuthenticatorCloser
}
