package model

import (
	"net/http"

	"github.com/sashabaranov/go-openai"
)

// Option 是 OpenAI 兼容客户端的可选配置项。
type Option func(*options)

type options struct {
	httpClient    *http.Client
	configMutator func(*openai.ClientConfig)
}

// WithHTTPClient 设置自定义 HTTP 客户端。
func WithHTTPClient(httpClient *http.Client) Option {
	return func(o *options) {
		o.httpClient = httpClient
	}
}

// WithConfigMutator 设置创建客户端前的配置调整函数。
func WithConfigMutator(mutator func(*openai.ClientConfig)) Option {
	return func(o *options) {
		o.configMutator = mutator
	}
}
