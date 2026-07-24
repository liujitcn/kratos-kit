package eino

import "github.com/cloudwego/eino-ext/components/model/agenticopenai"

// Option 是 Eino AgenticModel 的可选配置项。
type Option func(*options)

type options struct {
	chatConfigMutator      func(*agenticopenai.ChatConfig)
	responsesConfigMutator func(*agenticopenai.ResponsesConfig)
}

// WithChatConfigMutator 设置创建 Agentic ChatModel 前的配置调整函数。
func WithChatConfigMutator(mutator func(*agenticopenai.ChatConfig)) Option {
	return func(o *options) {
		o.chatConfigMutator = mutator
	}
}

// WithResponsesConfigMutator 设置创建 ResponsesModel 前的配置调整函数。
func WithResponsesConfigMutator(mutator func(*agenticopenai.ResponsesConfig)) Option {
	return func(o *options) {
		o.responsesConfigMutator = mutator
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
