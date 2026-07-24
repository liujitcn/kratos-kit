package langchaingo

import (
	"net/http"

	lcOllama "github.com/tmc/langchaingo/llms/ollama"
	lcOpenAI "github.com/tmc/langchaingo/llms/openai"
)

// Option 是 LangChainGo 模型客户端的可选配置项。
type Option func(*options)

type options struct {
	openAIOpts []lcOpenAI.Option
	ollamaOpts []lcOllama.Option
	httpClient *http.Client
}

// WithOpenAIOptions 追加 LangChainGo OpenAI 原生选项。
func WithOpenAIOptions(opts ...lcOpenAI.Option) Option {
	return func(o *options) {
		o.openAIOpts = append(o.openAIOpts, opts...)
	}
}

// WithOllamaOptions 追加 LangChainGo Ollama 原生选项。
func WithOllamaOptions(opts ...lcOllama.Option) Option {
	return func(o *options) {
		o.ollamaOpts = append(o.ollamaOpts, opts...)
	}
}

// WithHTTPClient 设置自定义 HTTP 客户端。
func WithHTTPClient(httpClient *http.Client) Option {
	return func(o *options) {
		o.httpClient = httpClient
	}
}

// applyOptions 应用可选配置项。
func applyOptions(opts []Option) *options {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}
