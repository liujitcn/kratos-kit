package langchaingo

import (
	"github.com/tmc/langchaingo/chains"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/memory"
	"github.com/tmc/langchaingo/prompts"
	"github.com/tmc/langchaingo/schema"
)

// NewLLMChain 创建一个 LLM 链。
func NewLLMChain(llm llms.Model, prompt prompts.FormatPrompter, opts ...chains.ChainCallOption) *chains.LLMChain {
	return chains.NewLLMChain(llm, prompt, opts...)
}

// NewConversationChain 创建一个带对话记忆的对话链。
func NewConversationChain(llm llms.Model, mem schema.Memory) chains.LLMChain {
	if mem == nil {
		mem = memory.NewConversationBuffer()
	}
	return chains.NewConversation(llm, mem)
}

// NewSequentialChain 创建一个顺序链。
func NewSequentialChain(c []chains.Chain, inputKeys []string, outputKeys []string, opts ...chains.SequentialChainOption) (*chains.SequentialChain, error) {
	return chains.NewSequentialChain(c, inputKeys, outputKeys, opts...)
}

// NewSimpleSequentialChain 创建一个简单顺序链。
func NewSimpleSequentialChain(c []chains.Chain) (*chains.SimpleSequentialChain, error) {
	return chains.NewSimpleSequentialChain(c)
}

// NewStuffDocumentsChain 创建一个文档填充链。
func NewStuffDocumentsChain(llmChain *chains.LLMChain) chains.StuffDocuments {
	return chains.NewStuffDocuments(llmChain)
}

// LoadStuffSummarization 加载文档填充摘要链。
func LoadStuffSummarization(llm llms.Model) chains.StuffDocuments {
	return chains.LoadStuffSummarization(llm)
}

// LoadRefineSummarization 加载迭代精炼摘要链。
func LoadRefineSummarization(llm llms.Model) chains.RefineDocuments {
	return chains.LoadRefineSummarization(llm)
}

// LoadMapReduceSummarization 加载 MapReduce 摘要链。
func LoadMapReduceSummarization(llm llms.Model) chains.MapReduceDocuments {
	return chains.LoadMapReduceSummarization(llm)
}
