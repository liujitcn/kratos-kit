package langchaingo

import (
	"github.com/tmc/langchaingo/agents"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/tools"
)

// NewOneShotAgent 创建一个 ReAct 风格的单次 Agent。
func NewOneShotAgent(llm llms.Model, agentTools []tools.Tool, opts ...agents.Option) *agents.OneShotZeroAgent {
	return agents.NewOneShotAgent(llm, agentTools, opts...)
}

// NewConversationalAgent 创建一个对话式 Agent。
func NewConversationalAgent(llm llms.Model, agentTools []tools.Tool, opts ...agents.Option) *agents.ConversationalAgent {
	return agents.NewConversationalAgent(llm, agentTools, opts...)
}

// NewOpenAIFunctionsAgent 创建一个基于 OpenAI Function Calling 的 Agent。
func NewOpenAIFunctionsAgent(llm llms.Model, agentTools []tools.Tool, opts ...agents.Option) *agents.OpenAIFunctionsAgent {
	return agents.NewOpenAIFunctionsAgent(llm, agentTools, opts...)
}

// NewExecutor 创建一个 Agent 执行器。
func NewExecutor(agent agents.Agent, opts ...agents.Option) *agents.Executor {
	return agents.NewExecutor(agent, opts...)
}

// NewOneShotExecutor 创建 OneShot Agent 并包装为 Executor。
func NewOneShotExecutor(llm llms.Model, agentTools []tools.Tool, opts ...agents.Option) *agents.Executor {
	agent := agents.NewOneShotAgent(llm, agentTools, opts...)
	return agents.NewExecutor(agent, opts...)
}

// NewConversationalExecutor 创建 Conversational Agent 并包装为 Executor。
func NewConversationalExecutor(llm llms.Model, agentTools []tools.Tool, opts ...agents.Option) *agents.Executor {
	agent := agents.NewConversationalAgent(llm, agentTools, opts...)
	return agents.NewExecutor(agent, opts...)
}

// NewOpenAIFunctionsExecutor 创建 OpenAI Functions Agent 并包装为 Executor。
func NewOpenAIFunctionsExecutor(llm llms.Model, agentTools []tools.Tool, opts ...agents.Option) *agents.Executor {
	agent := agents.NewOpenAIFunctionsAgent(llm, agentTools, opts...)
	return agents.NewExecutor(agent, opts...)
}
