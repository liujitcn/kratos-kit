// Package apikey 提供静态密钥或回调校验的 API Key 认证器。
package apikey

import (
	"context"
	"errors"

	"github.com/liujitcn/kratos-kit/auth/authn/engine"
)

var (
	// ErrMissingValidator 表示没有配置任何 API Key 校验方式。
	ErrMissingValidator = errors.New("apikey: keys or validator is required")
	// ErrMissingClientKey 表示没有可用于客户端请求的 API Key。
	ErrMissingClientKey = errors.New("apikey: client key is required")
)

// KeyValidator 校验 API Key 并返回关联声明。
type KeyValidator func(apiKey string) (map[string]any, bool)

// Option 配置 API Key 认证器。
type Option func(*options)

type options struct {
	keys      map[string]map[string]any
	validator KeyValidator
	clientKey string
}

// WithKeys 配置静态有效 API Key 集合。
func WithKeys(keys ...string) Option {
	return func(options *options) {
		if options.keys == nil {
			options.keys = make(map[string]map[string]any)
		}
		for _, key := range keys {
			options.keys[key] = nil
		}
	}
}

// WithKeyClaims 配置 API Key 及其关联声明。
func WithKeyClaims(apiKey string, claims map[string]any) Option {
	return func(options *options) {
		if options.keys == nil {
			options.keys = make(map[string]map[string]any)
		}
		options.keys[apiKey] = cloneClaims(claims)
	}
}

// WithValidator 配置外部 API Key 校验函数。
func WithValidator(validator KeyValidator) Option {
	return func(options *options) {
		options.validator = validator
	}
}

// WithClientKey 配置客户端请求使用的 API Key。
func WithClientKey(apiKey string) Option {
	return func(options *options) {
		options.clientKey = apiKey
	}
}

// Authenticator 校验 API Key。
type Authenticator struct {
	options *options
}

var _ engine.Authenticator = (*Authenticator)(nil)

// NewAuthenticator 创建 API Key 认证器。
func NewAuthenticator(opts ...Option) (engine.Authenticator, error) {
	options := &options{}
	for _, option := range opts {
		option(options)
	}
	if len(options.keys) == 0 && options.validator == nil {
		return nil, ErrMissingValidator
	}
	return &Authenticator{options: options}, nil
}

// Authenticate 从请求上下文读取 Bearer API Key 并校验。
func (a *Authenticator) Authenticate(ctx context.Context, contextType engine.ContextType, _ any) (*engine.AuthClaims, error) {
	token, err := engine.AuthFromMD(ctx, engine.BearerWord, contextType)
	if err != nil {
		return nil, engine.ErrMissingBearerToken
	}
	return a.AuthenticateToken(token)
}

// AuthenticateToken 校验 API Key 字符串。
func (a *Authenticator) AuthenticateToken(token string) (*engine.AuthClaims, error) {
	if a.options.validator != nil {
		claims, valid := a.options.validator(token)
		if !valid {
			return nil, engine.ErrUnauthenticated
		}
		authClaims := engine.AuthClaims(cloneClaims(claims))
		return &authClaims, nil
	}
	claims, ok := a.options.keys[token]
	if !ok {
		return nil, engine.ErrUnauthenticated
	}
	authClaims := engine.AuthClaims(cloneClaims(claims))
	return &authClaims, nil
}

// CreateIdentityWithContext 把客户端 API Key 写入请求上下文。
func (a *Authenticator) CreateIdentityWithContext(ctx context.Context, contextType engine.ContextType, claims engine.AuthClaims, _ any) (context.Context, error) {
	token, err := a.CreateIdentity(claims)
	if err != nil {
		return ctx, err
	}
	return engine.MDWithAuth(ctx, engine.BearerWord, token, contextType), nil
}

// CreateIdentity 返回显式配置的客户端 API Key。
func (a *Authenticator) CreateIdentity(engine.AuthClaims) (string, error) {
	if a.options.clientKey == "" {
		return "", ErrMissingClientKey
	}
	return a.options.clientKey, nil
}

// Close 释放认证器资源。
func (a *Authenticator) Close() {}

// cloneClaims 复制声明，避免调用方修改认证器内部静态数据。
func cloneClaims(claims map[string]any) map[string]any {
	cloned := make(map[string]any, len(claims))
	for key, value := range claims {
		cloned[key] = value
	}
	return cloned
}
