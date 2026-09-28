package model

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/sashabaranov/go-openai"
)

const defaultTimeout = 30 * time.Second

// NewClient 根据 AI 模型配置创建 OpenAI 兼容客户端。
func NewClient(cfg *ModelConfig, opts ...Option) (*openai.Client, error) {
	if cfg == nil {
		return nil, errors.New("ai model config is nil")
	}

	o := applyOptions(opts)

	switch cfg.Provider {
	case ProviderOpenAICompatible, ProviderOllama:
		return newCompatibleClient(cfg, o)
	default:
		return nil, fmt.Errorf("unsupported ai model provider: %s", cfg.Provider)
	}
}

// newCompatibleClient 创建 OpenAI 兼容客户端。
func newCompatibleClient(cfg *ModelConfig, o *options) (*openai.Client, error) {
	clientConfig := openai.DefaultConfig(cfg.ResolvedAPIKey())
	clientConfig.BaseURL = cfg.ResolvedBaseURL()
	if cfg.Provider == ProviderOpenAICompatible && cfg.Organization != "" {
		clientConfig.OrgID = cfg.Organization
	}
	applyClientConfig(cfg, o, &clientConfig)

	return openai.NewClientWithConfig(clientConfig), nil
}

// applyClientConfig 应用 HTTP 客户端、超时与用户自定义配置。
func applyClientConfig(cfg *ModelConfig, o *options, clientConfig *openai.ClientConfig) {
	if o.httpClient != nil {
		clientConfig.HTTPClient = o.httpClient
	} else {
		clientConfig.HTTPClient = &http.Client{Timeout: timeout(cfg)}
	}
	if o.configMutator != nil {
		o.configMutator(clientConfig)
	}
}

// timeout 返回 AI 请求超时时间。
func timeout(cfg *ModelConfig) time.Duration {
	if cfg.TimeoutSeconds > 0 {
		return time.Duration(cfg.TimeoutSeconds) * time.Second
	}
	return defaultTimeout
}

// applyOptions 应用可选配置项。
func applyOptions(opts []Option) *options {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}
