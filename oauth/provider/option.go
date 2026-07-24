package provider

import "net/url"

// GrantType 表示 GetToken 使用的 OAuth 授权类型。
type GrantType string

const (
	// GrantTypeAuthorizationCode 表示使用授权码换取 Token。
	GrantTypeAuthorizationCode GrantType = "authorization_code"

	// GrantTypeClientCredentials 表示使用客户端凭证获取 Token。
	GrantTypeClientCredentials GrantType = "client_credentials"
)

// Options 表示 OAuth 授权地址与换取 Token 的可选参数。
type Options struct {
	Scopes      []string
	RedirectURI string
	Params      url.Values
	PKCE        *PKCEChallenge
	GrantType   GrantType
}

// Option 用于配置 OAuth 授权地址或换取 Token 的参数。
type Option func(*Options)

// WithScopes 覆盖授权地址使用的 scope。
func WithScopes(scopes ...string) Option {
	return func(o *Options) {
		o.Scopes = scopes
	}
}

// WithRedirectURI 覆盖授权地址使用的 redirect uri。
func WithRedirectURI(uri string) Option {
	return func(o *Options) {
		o.RedirectURI = uri
	}
}

// WithParam 追加 OAuth 请求参数。
func WithParam(key, value string) Option {
	return func(o *Options) {
		if o.Params == nil {
			o.Params = url.Values{}
		}
		o.Params.Set(key, value)
	}
}

// WithPKCE 为授权地址与换取 Token 追加 PKCE 参数。
func WithPKCE(challenge PKCEChallenge) Option {
	return func(o *Options) {
		o.PKCE = &challenge
	}
}

// WithGrantType 指定 GetToken 使用的 OAuth 授权类型。
func WithGrantType(grantType GrantType) Option {
	return func(o *Options) {
		o.GrantType = grantType
	}
}

// ApplyOptions 合并 OAuth 可选参数。
func ApplyOptions(opts ...Option) Options {
	o := Options{
		Params:    url.Values{},
		GrantType: GrantTypeAuthorizationCode,
	}
	for _, opt := range opts {
		// 空 Option 不影响其他有效配置项。
		if opt != nil {
			opt(&o)
		}
	}
	return o
}
