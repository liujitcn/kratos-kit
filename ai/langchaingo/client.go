package langchaingo

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/liujitcn/kratos-kit/ai/model"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/tmc/langchaingo/llms/openai"
)

const defaultTimeout = 30 * time.Second

// NewModel 根据 AI 模型配置创建 LangChainGo LLM 客户端。
func NewModel(cfg *model.ModelConfig, opts ...Option) (llms.Model, error) {
	if cfg == nil {
		return nil, errors.New("ai model config is nil")
	}

	o := applyOptions(opts)

	switch cfg.Provider {
	case model.ProviderOpenAICompatible:
		return newCloudModel(cfg, o)
	case model.ProviderOllama:
		return newLocalModel(cfg, o)
	default:
		return nil, fmt.Errorf("unsupported ai model provider: %s", cfg.Provider)
	}
}

// newCloudModel 创建云端 OpenAI 兼容模型客户端。
func newCloudModel(cfg *model.ModelConfig, o *options) (llms.Model, error) {
	opts := []openai.Option{
		openai.WithToken(cfg.APIKey),
		openai.WithModel(cfg.ModelName),
		openai.WithHTTPClient(httpClient(cfg, o)),
	}
	if cfg.BaseURL != "" {
		opts = append(opts, openai.WithBaseURL(cfg.BaseURL))
	}
	opts = append(opts, o.openAIOpts...)

	return openai.New(opts...)
}

// newLocalModel 创建本地 Ollama 模型客户端。
func newLocalModel(cfg *model.ModelConfig, o *options) (llms.Model, error) {
	serverURL := strings.TrimSuffix(strings.TrimSuffix(cfg.ResolvedBaseURL(), "/"), "/v1")
	opts := []ollama.Option{
		ollama.WithModel(cfg.ModelName),
		ollama.WithServerURL(serverURL),
		ollama.WithHTTPClient(httpClient(cfg, o)),
	}
	opts = append(opts, o.ollamaOpts...)

	return ollama.New(opts...)
}

// httpClient 返回模型请求使用的 HTTP 客户端。
func httpClient(cfg *model.ModelConfig, o *options) *http.Client {
	if o.httpClient != nil {
		return o.httpClient
	}
	timeout := defaultTimeout
	if cfg.TimeoutSeconds > 0 {
		timeout = time.Duration(cfg.TimeoutSeconds) * time.Second
	}
	return &http.Client{Timeout: timeout}
}
