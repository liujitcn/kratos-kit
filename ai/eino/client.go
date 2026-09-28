package eino

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cloudwego/eino-ext/components/model/agenticopenai"
	"github.com/cloudwego/eino/components/model"
	modelconfig "github.com/liujitcn/kratos-kit/ai/model"
)

// NewChatModel 根据 AI 模型配置创建基于 Chat Completions API 的 Eino AgenticModel。
func NewChatModel(ctx context.Context, cfg *modelconfig.ModelConfig, opts ...Option) (model.AgenticModel, error) {
	if cfg == nil {
		return nil, errors.New("ai model config is nil")
	}
	if !supportedProvider(cfg.Provider) {
		return nil, fmt.Errorf("unsupported ai model provider: %s", cfg.Provider)
	}
	return newChatModel(ctx, cfg, applyOptions(opts))
}

// NewResponsesModel 根据 AI 模型配置创建基于 Responses API 的 Eino AgenticModel。
func NewResponsesModel(ctx context.Context, cfg *modelconfig.ModelConfig, opts ...Option) (model.AgenticModel, error) {
	if cfg == nil {
		return nil, errors.New("ai model config is nil")
	}
	if !supportedProvider(cfg.Provider) {
		return nil, fmt.Errorf("unsupported ai model provider: %s", cfg.Provider)
	}
	return newResponsesModel(ctx, cfg, applyOptions(opts))
}

// newChatModel 创建使用统一基础地址的 Agentic ChatModel。
func newChatModel(ctx context.Context, cfg *modelconfig.ModelConfig, o *options) (model.AgenticModel, error) {
	modelConfig := &agenticopenai.ChatConfig{
		APIKey:  cfg.ResolvedAPIKey(),
		BaseURL: cfg.ResolvedBaseURL(),
		Model:   cfg.ModelName,
	}
	applyChatModelConfig(cfg, o, modelConfig)

	return agenticopenai.NewChatModel(ctx, modelConfig)
}

// newResponsesModel 创建使用统一基础地址的 Agentic ResponsesModel。
func newResponsesModel(ctx context.Context, cfg *modelconfig.ModelConfig, o *options) (model.AgenticModel, error) {
	modelConfig := &agenticopenai.ResponsesConfig{
		APIKey:  cfg.ResolvedAPIKey(),
		BaseURL: cfg.ResolvedBaseURL(),
		Model:   cfg.ModelName,
	}
	applyResponsesModelConfig(cfg, o, modelConfig)

	return agenticopenai.NewResponsesModel(ctx, modelConfig)
}

// applyChatModelConfig 应用 Chat Completions 模型公共配置。
func applyChatModelConfig(cfg *modelconfig.ModelConfig, o *options, modelConfig *agenticopenai.ChatConfig) {
	if cfg.TimeoutSeconds > 0 {
		modelConfig.Timeout = time.Duration(cfg.TimeoutSeconds) * time.Second
	}
	if cfg.Temperature > 0 {
		modelConfig.Temperature = new(float32)
		*modelConfig.Temperature = cfg.Temperature
	}
	if cfg.MaxTokens > 0 {
		modelConfig.MaxCompletionTokens = new(int)
		*modelConfig.MaxCompletionTokens = int(cfg.MaxTokens)
	}
	if o.chatConfigMutator != nil {
		o.chatConfigMutator(modelConfig)
	}
}

// applyResponsesModelConfig 应用 Responses 模型公共配置。
func applyResponsesModelConfig(cfg *modelconfig.ModelConfig, o *options, modelConfig *agenticopenai.ResponsesConfig) {
	if cfg.TimeoutSeconds > 0 {
		modelConfig.Timeout = new(time.Duration)
		*modelConfig.Timeout = time.Duration(cfg.TimeoutSeconds) * time.Second
	}
	if cfg.MaxRetries > 0 {
		modelConfig.MaxRetries = new(int)
		*modelConfig.MaxRetries = int(cfg.MaxRetries)
	}
	if cfg.Temperature > 0 {
		modelConfig.Temperature = new(float32)
		*modelConfig.Temperature = cfg.Temperature
	}
	if cfg.MaxTokens > 0 {
		modelConfig.MaxTokens = new(int)
		*modelConfig.MaxTokens = int(cfg.MaxTokens)
	}
	if o.responsesConfigMutator != nil {
		o.responsesConfigMutator(modelConfig)
	}
}

// supportedProvider 判断模型Provider是否受当前客户端支持。
func supportedProvider(provider modelconfig.Provider) bool {
	return provider == modelconfig.ProviderOpenAICompatible || provider == modelconfig.ProviderOllama
}
