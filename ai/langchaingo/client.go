package langchaingo

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/tmc/langchaingo/llms"
	lcOllama "github.com/tmc/langchaingo/llms/ollama"
	lcOpenAI "github.com/tmc/langchaingo/llms/openai"
)

const (
	defaultTimeout   = 30 * time.Second
	defaultLocalHost = "localhost"
	defaultLocalPort = 11434
)

// NewModel 根据 AI 模型配置创建 LangChainGo LLM 客户端。
func NewModel(cfg *configv1.AI_Model, opts ...Option) (llms.Model, error) {
	if cfg == nil {
		return nil, errors.New("ai model config is nil")
	}

	o := applyOptions(opts)

	switch cfg.GetType() {
	case configv1.AI_Model_CLOUD_MODEL:
		return newCloudModel(cfg, o)
	case configv1.AI_Model_LOCAL_MODEL:
		return newLocalModel(cfg, o)
	default:
		return nil, fmt.Errorf("unsupported ai model type: %v", cfg.GetType())
	}
}

// newCloudModel 创建云端 OpenAI 兼容模型客户端。
func newCloudModel(cfg *configv1.AI_Model, o *options) (llms.Model, error) {
	cloud := cfg.GetCloud()
	if cloud == nil {
		return nil, errors.New("ai cloud config is nil")
	}

	opts := []lcOpenAI.Option{
		lcOpenAI.WithToken(cloud.GetApiKey()),
		lcOpenAI.WithModel(cfg.GetModelName()),
		lcOpenAI.WithHTTPClient(httpClient(cfg, o)),
	}
	if cloud.GetBaseUrl() != "" {
		opts = append(opts, lcOpenAI.WithBaseURL(cloud.GetBaseUrl()))
	}
	opts = append(opts, o.openAIOpts...)

	return lcOpenAI.New(opts...)
}

// newLocalModel 创建本地 Ollama 模型客户端。
func newLocalModel(cfg *configv1.AI_Model, o *options) (llms.Model, error) {
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

	opts := []lcOllama.Option{
		lcOllama.WithModel(cfg.GetModelName()),
		lcOllama.WithServerURL(fmt.Sprintf("http://%s:%d", host, port)),
		lcOllama.WithHTTPClient(httpClient(cfg, o)),
	}
	opts = append(opts, o.ollamaOpts...)

	return lcOllama.New(opts...)
}

// httpClient 返回模型请求使用的 HTTP 客户端。
func httpClient(cfg *configv1.AI_Model, o *options) *http.Client {
	if o.httpClient != nil {
		return o.httpClient
	}
	timeout := defaultTimeout
	if cfg.GetTimeoutSeconds() > 0 {
		timeout = time.Duration(cfg.GetTimeoutSeconds()) * time.Second
	}
	return &http.Client{Timeout: timeout}
}
