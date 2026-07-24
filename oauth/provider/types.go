package provider

import "context"

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
