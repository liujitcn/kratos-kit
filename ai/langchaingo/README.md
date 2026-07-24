# ai/langchaingo 模块说明

`ai/langchaingo` 基于 LangChainGo 封装 LLM 创建、Agent、Chain、Memory、Embedding 与 VectorStore 常用入口。

## 配置

模块使用 `configv1.AI_Model`。云端模型配置：

```yaml
ai:
  model:
    type: CLOUD_MODEL
    model_name: gpt-4o
    timeout_seconds: 60
    cloud:
      api_key: sk-xxx
      base_url: https://api.openai.com/v1
```

本地 Ollama 配置：

```yaml
ai:
  model:
    type: LOCAL_MODEL
    model_name: llama3
    local:
      host: 127.0.0.1
      port: 11434
```

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
	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/tmc/langchaingo/tools"
)

func Example(ctx context.Context, agentTools []tools.Tool) error {
	cfg := &configv1.AI_Model{
		Type:      configv1.AI_Model_CLOUD_MODEL,
		ModelName: "gpt-4o",
		Cloud: &configv1.AI_Model_CloudConfig{
			ApiKey:  "sk-xxx",
			BaseUrl: "https://api.openai.com/v1",
		},
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
