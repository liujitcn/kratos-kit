package oauth

import (
	"slices"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/liujitcn/kratos-kit/oauth/dingtalk"
	"github.com/liujitcn/kratos-kit/oauth/feishu"
	"github.com/liujitcn/kratos-kit/oauth/gitee"
	"github.com/liujitcn/kratos-kit/oauth/github"
	"github.com/liujitcn/kratos-kit/oauth/google"
	"github.com/liujitcn/kratos-kit/oauth/provider"
	"github.com/liujitcn/kratos-kit/oauth/wechat"
	"github.com/liujitcn/kratos-kit/oauth/wechatmini"
	"github.com/liujitcn/kratos-kit/oauth/wechatmp"
	"github.com/liujitcn/kratos-kit/oauth/wechatwork"
)

// Manager 管理根据配置创建的 OAuth Provider。
type Manager struct {
	providers map[Type]provider.OAuth
}

// NewManager 创建 OAuth 管理器，并根据配置实例化 Provider。
func NewManager(config *configv1.OAuth) (*Manager, error) {
	manager := &Manager{
		providers: make(map[Type]provider.OAuth, len(config.GetProviders())),
	}
	for name, providerConfig := range config.GetProviders() {
		providerName := Type(name)
		// 只实例化配置完整的 Provider，避免无效配置影响业务侧判断。
		if providerConfig.GetClientId() == "" || providerConfig.GetClientSecret() == "" {
			continue
		}
		// 根据配置名称创建当前组件已实现的 Provider。
		switch providerName {
		case Github:
			manager.providers[providerName] = github.New(providerConfig)
		case Gitee:
			manager.providers[providerName] = gitee.New(providerConfig)
		case Google:
			manager.providers[providerName] = google.New(providerConfig)
		case Wechat:
			manager.providers[providerName] = wechat.New(providerConfig)
		case WechatMP:
			manager.providers[providerName] = wechatmp.New(providerConfig)
		case WechatMini:
			manager.providers[providerName] = wechatmini.New(providerConfig)
		case WechatWork:
			manager.providers[providerName] = wechatwork.New(providerConfig)
		case DingTalk:
			manager.providers[providerName] = dingtalk.New(providerConfig)
		case Feishu:
			manager.providers[providerName] = feishu.New(providerConfig)
		default:
			// 未实现的 Provider 配置不进入管理器。
			continue
		}
	}
	return manager, nil
}

// Get 根据 Provider 名称获取 OAuth Provider。
func (m *Manager) Get(name Type) (provider.OAuth, error) {
	oauthProvider, ok := m.providers[name]
	if !ok {
		return nil, NewProviderNotFoundError(name)
	}
	return oauthProvider, nil
}

// Providers 返回当前配置完整且支持跳转 OAuth 授权的 Provider 名称。
func (m *Manager) Providers() []Type {
	providers := make([]Type, 0, len(m.providers))
	for name := range m.providers {
		// 只展示能够生成跳转授权地址的 Provider。
		if m.IsSupported(name) {
			providers = append(providers, name)
		}
	}
	slices.Sort(providers)
	return providers
}

// IsSupported 判断 Provider 是否支持跳转 OAuth 授权。
func (m *Manager) IsSupported(name Type) bool {
	oauthProvider, ok := m.providers[name]
	return ok && oauthProvider.AuthURL("") != ""
}
