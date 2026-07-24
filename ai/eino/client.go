package eino

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/cloudwego/eino-ext/components/model/agenticopenai"
	"github.com/cloudwego/eino/components/model"
	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
)

const (
	defaultLocalHost = "localhost"
	defaultLocalPort = 11434
	localBaseScheme  = "http"
)

// NewChatModel 根据 AI 模型配置创建基于 Chat Completions API 的 Eino AgenticModel。
func NewChatModel(ctx context.Context, cfg *configv1.AI_Model, opts ...Option) (model.AgenticModel, error) {
	if cfg == nil {
		return nil, errors.New("ai model config is nil")
	}

	o := applyOptions(opts)

	switch cfg.GetType() {
	case configv1.AI_Model_CLOUD_MODEL:
		return newCloudChatModel(ctx, cfg, o)
	case configv1.AI_Model_LOCAL_MODEL:
		return newLocalChatModel(ctx, cfg, o)
	default:
		return nil, fmt.Errorf("unsupported ai model type: %v", cfg.GetType())
	}
}

// NewResponsesModel 根据 AI 模型配置创建基于 Responses API 的 Eino AgenticModel。
func NewResponsesModel(ctx context.Context, cfg *configv1.AI_Model, opts ...Option) (model.AgenticModel, error) {
	if cfg == nil {
		return nil, errors.New("ai model config is nil")
	}

	o := applyOptions(opts)

	switch cfg.GetType() {
	case configv1.AI_Model_CLOUD_MODEL:
		return newCloudResponsesModel(ctx, cfg, o)
	case configv1.AI_Model_LOCAL_MODEL:
		return newLocalResponsesModel(ctx, cfg, o)
	default:
		return nil, fmt.Errorf("unsupported ai model type: %v", cfg.GetType())
	}
}

// newCloudChatModel 创建云端 OpenAI 兼容 Agentic ChatModel。
func newCloudChatModel(ctx context.Context, cfg *configv1.AI_Model, o *options) (model.AgenticModel, error) {
	cloud := cfg.GetCloud()
	if cloud == nil {
		return nil, errors.New("ai cloud config is nil")
	}

	modelConfig := &agenticopenai.ChatConfig{
		APIKey: cloud.GetApiKey(),
		Model:  cfg.GetModelName(),
	}
	if cloud.GetBaseUrl() != "" {
		modelConfig.BaseURL = cloud.GetBaseUrl()
	}
	applyChatModelConfig(cfg, o, modelConfig)

	return agenticopenai.NewChatModel(ctx, modelConfig)
}

// newLocalChatModel 创建本地 Ollama OpenAI 兼容 Agentic ChatModel。
func newLocalChatModel(ctx context.Context, cfg *configv1.AI_Model, o *options) (model.AgenticModel, error) {
	local := cfg.GetLocal()
	if local == nil {
		return nil, errors.New("ai local config is nil")
	}

	modelConfig := &agenticopenai.ChatConfig{
		APIKey:  "ollama",
		BaseURL: buildLocalBaseURL(local),
		Model:   cfg.GetModelName(),
	}
	applyChatModelConfig(cfg, o, modelConfig)

	return agenticopenai.NewChatModel(ctx, modelConfig)
}

// newCloudResponsesModel 创建云端 OpenAI Responses AgenticModel。
func newCloudResponsesModel(ctx context.Context, cfg *configv1.AI_Model, o *options) (model.AgenticModel, error) {
	cloud := cfg.GetCloud()
	if cloud == nil {
		return nil, errors.New("ai cloud config is nil")
	}

	modelConfig := &agenticopenai.ResponsesConfig{
		APIKey: cloud.GetApiKey(),
		Model:  cfg.GetModelName(),
	}
	if cloud.GetBaseUrl() != "" {
		modelConfig.BaseURL = cloud.GetBaseUrl()
	}
	applyResponsesModelConfig(cfg, o, modelConfig)

	return agenticopenai.NewResponsesModel(ctx, modelConfig)
}

// newLocalResponsesModel 创建本地 Ollama OpenAI 兼容 Responses AgenticModel。
func newLocalResponsesModel(ctx context.Context, cfg *configv1.AI_Model, o *options) (model.AgenticModel, error) {
	local := cfg.GetLocal()
	if local == nil {
		return nil, errors.New("ai local config is nil")
	}

	modelConfig := &agenticopenai.ResponsesConfig{
		APIKey:  "ollama",
		BaseURL: buildLocalBaseURL(local),
		Model:   cfg.GetModelName(),
	}
	applyResponsesModelConfig(cfg, o, modelConfig)

	return agenticopenai.NewResponsesModel(ctx, modelConfig)
}

// applyChatModelConfig 应用 Chat Completions 模型公共配置。
func applyChatModelConfig(cfg *configv1.AI_Model, o *options, modelConfig *agenticopenai.ChatConfig) {
	if cfg.GetTimeoutSeconds() > 0 {
		modelConfig.Timeout = time.Duration(cfg.GetTimeoutSeconds()) * time.Second
	}
	if cfg.GetTemperature() > 0 {
		modelConfig.Temperature = new(float32)
		*modelConfig.Temperature = cfg.GetTemperature()
	}
	if cfg.GetMaxTokens() > 0 {
		modelConfig.MaxCompletionTokens = new(int)
		*modelConfig.MaxCompletionTokens = int(cfg.GetMaxTokens())
	}
	if o.chatConfigMutator != nil {
		o.chatConfigMutator(modelConfig)
	}
}

// applyResponsesModelConfig 应用 Responses 模型公共配置。
func applyResponsesModelConfig(cfg *configv1.AI_Model, o *options, modelConfig *agenticopenai.ResponsesConfig) {
	if cfg.GetTimeoutSeconds() > 0 {
		modelConfig.Timeout = new(time.Duration)
		*modelConfig.Timeout = time.Duration(cfg.GetTimeoutSeconds()) * time.Second
	}
	if cfg.GetMaxRetries() > 0 {
		modelConfig.MaxRetries = new(int)
		*modelConfig.MaxRetries = int(cfg.GetMaxRetries())
	}
	if cfg.GetTemperature() > 0 {
		modelConfig.Temperature = new(float32)
		*modelConfig.Temperature = cfg.GetTemperature()
	}
	if cfg.GetMaxTokens() > 0 {
		modelConfig.MaxTokens = new(int)
		*modelConfig.MaxTokens = int(cfg.GetMaxTokens())
	}
	if o.responsesConfigMutator != nil {
		o.responsesConfigMutator(modelConfig)
	}
}

// buildLocalBaseURL 根据本地模型配置生成 OpenAI 兼容 API 基础地址。
func buildLocalBaseURL(local *configv1.AI_Model_LocalConfig) string {
	host := local.GetHost()
	if host == "" {
		host = defaultLocalHost
	}
	port := local.GetPort()
	if port == 0 {
		port = defaultLocalPort
	}

	baseURL := url.URL{
		Scheme: localBaseScheme,
		Host:   net.JoinHostPort(host, strconv.Itoa(int(port))),
		Path:   "/v1",
	}
	return baseURL.String()
}
