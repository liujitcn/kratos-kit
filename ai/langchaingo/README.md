# ai/langchaingo 模块说明

`ai/langchaingo` 基于 LangChainGo 封装 LLM 创建、Agent、Chain、Memory、Embedding 与 VectorStore 常用入口。

## 运行时参数

模型客户端使用 `ai/model.ModelConfig` 接收运行时参数，不读取启动 YAML 配置。

## API

| 函数 | 说明 |
|------|------|
| `NewModel(cfg, opts...) (llms.Model, error)` | 根据配置创建 LangChainGo LLM |
| `WithOpenAIOptions(opts...) Option` | 追加 OpenAI 原生选项 |
| `WithOllamaOptions(opts...) Option` | 追加 Ollama 原生选项 |
| `WithHTTPClient(client) Option` | 设置自定义 HTTP 客户端 |
| `NewOpenAIFunctionsExecutor(llm, tools, opts...)` | 创建 OpenAI Functions Agent 执行器 |

## 使用

```go
package example

import (
	"context"

	aiLC "github.com/liujitcn/kratos-kit/ai/langchaingo"
	modelconfig "github.com/liujitcn/kratos-kit/ai/model"
	"github.com/tmc/langchaingo/tools"
)

func Example(ctx context.Context, agentTools []tools.Tool) error {
	cfg := &modelconfig.ModelConfig{
		Provider:  modelconfig.ProviderOpenAICompatible,
		ModelName: "gpt-4o",
		APIKey:    "sk-xxx",
		BaseURL:   "https://api.openai.com/v1",
	}

	llm, err := aiLC.NewModel(cfg)
	if err != nil {
		return err
	}

	executor := aiLC.NewOpenAIFunctionsExecutor(llm, agentTools)
	_, err = executor.Call(ctx, "今天北京天气如何？")
	return err
}
```
