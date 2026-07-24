package model

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/sashabaranov/go-openai"
)

const (
	defaultTimeout   = 30 * time.Second
	defaultLocalHost = "localhost"
	defaultLocalPort = 11434
)

// NewClient 根据 AI 模型配置创建 OpenAI 兼容客户端。
func NewClient(cfg *configv1.AI_Model, opts ...Option) (*openai.Client, error) {
	if cfg == nil {
		return nil, errors.New("ai model config is nil")
	}

	o := applyOptions(opts)

	switch cfg.GetType() {
	case configv1.AI_Model_CLOUD_MODEL:
		return newCloudClient(cfg, o)
	case configv1.AI_Model_LOCAL_MODEL:
		return newLocalClient(cfg, o)
	default:
		return nil, fmt.Errorf("unsupported ai model type: %v", cfg.GetType())
	}
}

// newCloudClient 创建云端 OpenAI 兼容客户端。
func newCloudClient(cfg *configv1.AI_Model, o *options) (*openai.Client, error) {
	cloud := cfg.GetCloud()
	if cloud == nil {
		return nil, errors.New("ai cloud config is nil")
	}

	clientConfig := openai.DefaultConfig(cloud.GetApiKey())
	if cloud.GetBaseUrl() != "" {
		clientConfig.BaseURL = cloud.GetBaseUrl()
	}
	if cloud.GetOrganization() != "" {
		clientConfig.OrgID = cloud.GetOrganization()
	}
	applyClientConfig(cfg, o, &clientConfig)

	return openai.NewClientWithConfig(clientConfig), nil
}

// newLocalClient 创建本地 Ollama OpenAI 兼容客户端。
func newLocalClient(cfg *configv1.AI_Model, o *options) (*openai.Client, error) {
	local := cfg.GetLocal()
	if local == nil {
		return nil, errors.New("ai local config is nil")
	}

	host := local.GetHost()
	if host == "" {
		host = defaultLocalHost
	}
	port := local.GetPort()
	if port == 0 {
		port = defaultLocalPort
	}

	clientConfig := openai.DefaultConfig("ollama")
	clientConfig.BaseURL = fmt.Sprintf("http://%s:%d/v1", host, port)
	applyClientConfig(cfg, o, &clientConfig)

	return openai.NewClientWithConfig(clientConfig), nil
}

// applyClientConfig 应用 HTTP 客户端、超时与用户自定义配置。
func applyClientConfig(cfg *configv1.AI_Model, o *options, clientConfig *openai.ClientConfig) {
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
func timeout(cfg *configv1.AI_Model) time.Duration {
	if cfg.GetTimeoutSeconds() > 0 {
		return time.Duration(cfg.GetTimeoutSeconds()) * time.Second
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
