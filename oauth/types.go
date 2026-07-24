package oauth

import "github.com/liujitcn/kratos-kit/oauth/provider"

// Type OAuth Provider 类型枚举。
type Type = provider.Type

const (
	// Github 表示 GitHub OAuth Provider。
	Github = provider.Github

	// Gitee 表示 Gitee OAuth Provider。
	Gitee = provider.Gitee

	// Google 表示 Google OAuth Provider。
	Google = provider.Google

	// Wechat 表示微信开放平台 OAuth Provider。
	Wechat = provider.Wechat

	// WechatMP 表示微信公众号网页授权 OAuth Provider。
	WechatMP = provider.WechatMP

	// WechatMini 表示微信小程序登录 Provider。
	WechatMini = provider.WechatMini

	// WechatWork 表示企业微信 OAuth Provider。
	WechatWork = provider.WechatWork

	// DingTalk 表示钉钉 OAuth Provider。
	DingTalk = provider.DingTalk

	// Feishu 表示飞书 OAuth Provider。
	Feishu = provider.Feishu
)
