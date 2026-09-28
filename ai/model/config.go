package model

// Provider 表示 AI 模型客户端使用的 Provider 类型。
type Provider string

const (
	// ProviderOpenAICompatible 表示 OpenAI 兼容接口。
	ProviderOpenAICompatible Provider = "openai_compatible"
	// ProviderOllama 表示 Ollama 接口。
	ProviderOllama Provider = "ollama"
)

// APIType 表示模型 API 协议类型。
type APIType int32

const (
	// APITypeUnspecified 表示未指定协议类型。
	APITypeUnspecified APIType = 0
	// APITypeChatCompletions 表示 Chat Completions 协议。
	APITypeChatCompletions APIType = 1
	// APITypeResponses 表示 Responses 协议。
	APITypeResponses APIType = 2
)

// ModelConfig 描述运行时 AI 模型客户端参数，不属于应用启动配置。
type ModelConfig struct {
	// Provider 是模型接口类型。
	Provider Provider
	// ModelName 是 Provider 中的模型名称。
	ModelName string
	// APIKey 是模型 API 密钥。
	APIKey string
	// BaseURL 是模型 API 基础地址。
	BaseURL string
	// Organization 是可选的组织标识。
	Organization string
	// Temperature 是生成温度。
	Temperature float32
	// MaxTokens 是模型最大输出 Token 数。
	MaxTokens int32
	// TimeoutSeconds 是请求超时秒数。
	TimeoutSeconds int32
	// MaxRetries 是请求最大重试次数。
	MaxRetries int32
	// APIType 是模型 API 协议类型。
	APIType APIType
}

// ResolvedAPIKey 返回模型客户端实际使用的 API Key。
func (c *ModelConfig) ResolvedAPIKey() string {
	if c.Provider == ProviderOllama && c.APIKey == "" {
		return "ollama"
	}
	return c.APIKey
}

// ResolvedBaseURL 返回模型客户端实际使用的基础地址。
func (c *ModelConfig) ResolvedBaseURL() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	if c.Provider == ProviderOllama {
		return "http://localhost:11434/v1"
	}
	if c.Provider == ProviderOpenAICompatible {
		return "https://api.openai.com/v1"
	}
	return ""
}
