package provider

import "context"

// Parameters 表示 Provider 厂商扩展参数的结构化 JSON 对象。
type Parameters map[string]any

// Config 描述 OAuth Provider 的标准参数和厂商扩展参数。
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	Scopes       []string
	Parameters   Parameters
}

// GetClientId 返回 OAuth Client ID。
func (c *Config) GetClientId() string {
	if c == nil {
		return ""
	}
	return c.ClientID
}

// GetClientSecret 返回 OAuth Client Secret。
func (c *Config) GetClientSecret() string {
	if c == nil {
		return ""
	}
	return c.ClientSecret
}

// GetRedirectUri 返回 OAuth 回调地址。
func (c *Config) GetRedirectUri() string {
	if c == nil {
		return ""
	}
	return c.RedirectURI
}

// GetScopes 返回 OAuth Scope 列表。
func (c *Config) GetScopes() []string {
	if c == nil {
		return nil
	}
	return c.Scopes
}

// GetParameters 返回厂商扩展参数。
func (c *Config) GetParameters() Parameters {
	if c == nil {
		return nil
	}
	return c.Parameters
}

// Type OAuth Provider 类型枚举。
type Type string

const (
	// Github 表示 GitHub OAuth Provider。
	Github Type = "github"

	// Gitee 表示 Gitee OAuth Provider。
	Gitee Type = "gitee"

	// Google 表示 Google OAuth Provider。
	Google Type = "google"

	// Wechat 表示微信开放平台 OAuth Provider。
	Wechat Type = "wechat"

	// WechatMP 表示微信公众号网页授权 OAuth Provider。
	WechatMP Type = "wechatmp"

	// WechatMini 表示微信小程序登录 Provider。
	WechatMini Type = "wechatmini"

	// WechatWork 表示企业微信 OAuth Provider。
	WechatWork Type = "wechatwork"

	// DingTalk 表示钉钉 OAuth Provider。
	DingTalk Type = "dingtalk"

	// Feishu 表示飞书 OAuth Provider。
	Feishu Type = "feishu"
)

// OAuth 定义 OAuth Provider 通用能力。
type OAuth interface {
	Name() Type
	AuthURL(state string, opts ...Option) string
	GetToken(ctx context.Context, code string, opts ...Option) (*Token, error)
	GetUser(ctx context.Context, token *Token) (*User, error)
}

// Token 是 OAuth code 换取后的统一 Token 结构。
type Token struct {
	AccessToken  string
	RefreshToken string
	IDToken      string
	TokenType    string
	ExpiresIn    int64
	Scope        string

	// 部分平台在换 token 阶段会直接返回 openid/unionid。
	OpenID  string
	UnionID string

	Raw []byte
}

// User 是不同 OAuth Provider 用户信息的统一结构。
type User struct {
	Provider Type
	OpenID   string
	UnionID  string
	Username string
	Nickname string
	Email    string
	Avatar   string
	Raw      []byte
}

// PKCEChallenge 表示 OAuth PKCE 授权挑战参数。
type PKCEChallenge struct {
	Verifier  string
	Challenge string
	Method    string
}
