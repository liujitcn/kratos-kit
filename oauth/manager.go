package oauth

import (
	"slices"
	"sync"

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
	mutex     sync.RWMutex
	providers map[Type]provider.OAuth
}

// NewManager 创建 OAuth 管理器，并根据参数实例化 Provider。
func NewManager(configs map[Type]*provider.Config) (*Manager, error) {
	manager := &Manager{
		providers: make(map[Type]provider.OAuth, len(configs)),
	}
	for providerName, providerConfig := range configs {
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

// Replace 使用新参数整体替换 OAuth Provider 快照。
func (m *Manager) Replace(configs map[Type]*provider.Config) error {
	replacement, err := NewManager(configs)
	if err != nil {
		return err
	}
	m.mutex.Lock()
	m.providers = replacement.providers
	m.mutex.Unlock()
	return nil
}

// Get 根据 Provider 名称获取 OAuth Provider。
func (m *Manager) Get(name Type) (provider.OAuth, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	oauthProvider, ok := m.providers[name]
	if !ok {
		return nil, NewProviderNotFoundError(name)
	}
	return oauthProvider, nil
}

// Providers 返回当前配置完整且支持跳转 OAuth 授权的 Provider 名称。
func (m *Manager) Providers() []Type {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	providers := make([]Type, 0, len(m.providers))
	for name := range m.providers {
		// 只展示能够生成跳转授权地址的 Provider。
		if m.isSupported(name) {
			providers = append(providers, name)
		}
	}
	slices.Sort(providers)
	return providers
}

// IsSupported 判断 Provider 是否支持跳转 OAuth 授权。
func (m *Manager) IsSupported(name Type) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.isSupported(name)
}

// isSupported 判断当前读锁范围内的 Provider 是否支持跳转 OAuth 授权。
func (m *Manager) isSupported(name Type) bool {
	oauthProvider, ok := m.providers[name]
	return ok && oauthProvider.AuthURL("") != ""
}
