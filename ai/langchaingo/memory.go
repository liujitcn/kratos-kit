package langchaingo

import (
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/memory"
	"github.com/tmc/langchaingo/schema"
)

// NewChatMessageHistory 创建内存中的聊天消息历史记录。
func NewChatMessageHistory(opts ...memory.ChatMessageHistoryOption) *memory.ChatMessageHistory {
	return memory.NewChatMessageHistory(opts...)
}

// NewConversationBuffer 创建完整对话缓冲记忆。
func NewConversationBuffer(opts ...memory.ConversationBufferOption) *memory.ConversationBuffer {
	return memory.NewConversationBuffer(opts...)
}

// NewConversationWindowBuffer 创建窗口对话缓冲记忆。
func NewConversationWindowBuffer(windowSize int, opts ...memory.ConversationBufferOption) *memory.ConversationWindowBuffer {
	return memory.NewConversationWindowBuffer(windowSize, opts...)
}

// NewConversationTokenBuffer 创建基于 Token 数量的对话缓冲记忆。
func NewConversationTokenBuffer(llm llms.Model, maxTokenLimit int, opts ...memory.ConversationBufferOption) *memory.ConversationTokenBuffer {
	return memory.NewConversationTokenBuffer(llm, maxTokenLimit, opts...)
}

// NewSimpleMemory 创建空记忆。
func NewSimpleMemory() memory.Simple {
	return memory.NewSimple()
}

// WithChatHistory 设置自定义聊天历史存储后端。
func WithChatHistory(chatHistory schema.ChatMessageHistory) memory.ConversationBufferOption {
	return memory.WithChatHistory(chatHistory)
}

// WithReturnMessages 设置是否以消息对象形式返回。
func WithReturnMessages(returnMessages bool) memory.ConversationBufferOption {
	return memory.WithReturnMessages(returnMessages)
}

// WithMemoryKey 设置记忆变量键名。
func WithMemoryKey(memoryKey string) memory.ConversationBufferOption {
	return memory.WithMemoryKey(memoryKey)
}

// WithInputKey 设置输入键名。
func WithInputKey(inputKey string) memory.ConversationBufferOption {
	return memory.WithInputKey(inputKey)
}

// WithOutputKey 设置输出键名。
func WithOutputKey(outputKey string) memory.ConversationBufferOption {
	return memory.WithOutputKey(outputKey)
}

// WithHumanPrefix 设置用户消息前缀。
func WithHumanPrefix(humanPrefix string) memory.ConversationBufferOption {
	return memory.WithHumanPrefix(humanPrefix)
}

// WithAIPrefix 设置 AI 消息前缀。
func WithAIPrefix(aiPrefix string) memory.ConversationBufferOption {
	return memory.WithAIPrefix(aiPrefix)
}
