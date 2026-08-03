// Package translator 提供基于配置的统一翻译模块。
package translator

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	utilsTranslator "github.com/liujitcn/go-utils/translator"
	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
)

const defaultTimeout = 8 * time.Second

const (
	// Google 表示 Google 翻译 Provider。
	Google = "google"
	// Baidu 表示百度翻译 Provider。
	Baidu = "baidu"
	// Alibaba 表示阿里云翻译 Provider。
	Alibaba = "alibaba"
	// Volc 表示火山引擎翻译 Provider。
	Volc = "volc"
)

// Translator 复用 go-utils 提供的统一翻译接口。
type Translator = utilsTranslator.Translator

// Factory 定义按配置创建翻译 Provider 的工厂函数。
type Factory func(cfg *configv1.Translator, httpClient *http.Client) (Translator, error)

var (
	providerMu        sync.RWMutex
	providerFactories = map[string]Factory{}
)

// NewTranslator 创建配置指定的翻译 Provider；未启用时返回 nil。
func NewTranslator(cfg *configv1.Translator) Translator {
	provider, err := NewTranslatorWithError(cfg)
	if err != nil {
		return &errorTranslator{err: err}
	}
	return provider
}

// NewTranslatorWithError 创建配置指定的翻译 Provider，并返回配置或客户端初始化错误。
func NewTranslatorWithError(cfg *configv1.Translator) (Translator, error) {
	if cfg == nil || !cfg.GetEnabled() {
		return nil, nil
	}

	providerName := strings.ToLower(cfg.GetType())
	if providerName == "" {
		providerName = Google
	}
	factory, exists := providerFactory(providerName)
	if !exists {
		return nil, fmt.Errorf("translator: unsupported provider %q", providerName)
	}

	timeout := cfg.GetTimeout().AsDuration()
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return factory(cfg, &http.Client{Timeout: timeout})
}

// RegisterProvider 注册一个可由配置 type 选择的翻译 Provider。
func RegisterProvider(name string, factory Factory) error {
	providerName := strings.ToLower(name)
	if providerName == "" {
		return fmt.Errorf("translator: provider name is empty")
	}
	if factory == nil {
		return fmt.Errorf("translator: provider %q factory is nil", providerName)
	}

	providerMu.Lock()
	defer providerMu.Unlock()
	if _, exists := providerFactories[providerName]; exists {
		return fmt.Errorf("translator: provider %q is already registered", providerName)
	}
	providerFactories[providerName] = factory
	return nil
}

// MustRegisterProvider 注册 Provider，失败时直接触发 panic。
func MustRegisterProvider(name string, factory Factory) {
	if err := RegisterProvider(name, factory); err != nil {
		panic(err)
	}
}

// ListProviders 返回当前已注册的 Provider 名称。
func ListProviders() []string {
	providerMu.RLock()
	providers := make([]string, 0, len(providerFactories))
	for name := range providerFactories {
		providers = append(providers, name)
	}
	providerMu.RUnlock()
	sort.Strings(providers)
	return providers
}

// providerFactory 查询已注册的 Provider 工厂。
func providerFactory(name string) (Factory, bool) {
	providerMu.RLock()
	factory, exists := providerFactories[name]
	providerMu.RUnlock()
	return factory, exists
}

type errorTranslator struct {
	err error
}

func (t *errorTranslator) Translate(_ context.Context, _, _, _ string) (string, error) {
	return "", t.err
}
