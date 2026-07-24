# ai/eino 模块说明

`ai/eino` 基于 CloudWeGo Eino 封装 AgenticModel 创建、Chain/Graph 编排、Prompt 与 Tool 辅助方法。

模型实现使用 `components/model/agenticopenai`：

- `NewChatModel` 创建基于 `/v1/chat/completions` 的 `model.AgenticModel`
- `NewResponsesModel` 创建基于 `/v1/responses` 的 `model.AgenticModel`

## 配置

模块使用 `configv1.AI_Model`。云端模型配置：

```yaml
ai:
  model:
    type: CLOUD_MODEL
    model_name: gpt-4o
    temperature: 0.7
    max_tokens: 4096
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
    timeout_seconds: 120
    local:
      host: 127.0.0.1
      port: 11434
```

本地模型是否能使用 `NewResponsesModel` 取决于本地 OpenAI 兼容服务是否实现 `/v1/responses`。

## API

| 函数 | 说明 |
|------|------|
| `NewChatModel(ctx, cfg, opts...) (model.AgenticModel, error)` | 根据配置创建基于 Chat Completions API 的 AgenticModel |
| `NewResponsesModel(ctx, cfg, opts...) (model.AgenticModel, error)` | 根据配置创建基于 Responses API 的 AgenticModel |
| `WithChatConfigMutator(fn) Option` | 创建前修改 Eino OpenAI `agenticopenai.ChatConfig` |
| `WithResponsesConfigMutator(fn) Option` | 创建前修改 Eino OpenAI `agenticopenai.ResponsesConfig` |
| `NewChain[I, O](opts...)` | 创建链式编排 |
| `NewGraph[I, O](opts...)` | 创建 DAG 工作流 |
| `NewWorkflow[I, O](opts...)` | 创建依赖式工作流 |
| `NewToolNode(ctx, config)` | 创建工具执行节点 |
| `NewAgenticToolNode(ctx, config)` | 创建 Agentic 工具执行节点 |

## 使用

```go
package example

import (
	"context"

	"github.com/cloudwego/eino/schema"
	aiEino "github.com/liujitcn/kratos-kit/ai/eino"
	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
)

func Example(ctx context.Context) error {
	cfg := &configv1.AI_Model{
		Type:      configv1.AI_Model_CLOUD_MODEL,
		ModelName: "gpt-4o",
		Cloud: &configv1.AI_Model_CloudConfig{
			ApiKey:  "sk-xxx",
			BaseUrl: "https://api.openai.com/v1",
		},
	}

	responsesModel, err := aiEino.NewResponsesModel(ctx, cfg)
	if err != nil {
		return err
	}

	tpl := aiEino.FromAgenticMessages(schema.FString,
		aiEino.SystemAgenticMessage("你是一个{role}助手。"),
		aiEino.UserAgenticMessage("{question}"),
	)

	chain := aiEino.NewChain[map[string]any, *schema.AgenticMessage]()
	chain.AppendAgenticChatTemplate(tpl).AppendAgenticModel(responsesModel)

	_, err = aiEino.CompileChain(ctx, chain)
	return err
}
```
