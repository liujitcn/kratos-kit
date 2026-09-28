# ai/model 模块说明

`ai/model` 基于 `github.com/sashabaranov/go-openai` 封装 OpenAI 兼容客户端创建能力。

## 运行时参数

客户端使用 `ModelConfig` 接收 Provider、模型名称、密钥、基础地址和请求参数，不读取 `Bootstrap` 或 YAML。

## API

| 函数 | 说明 |
|------|------|
| `NewClient(cfg, opts...) (*openai.Client, error)` | 根据配置创建 OpenAI 兼容客户端 |
| `WithHTTPClient(client) Option` | 设置自定义 HTTP 客户端 |
| `WithConfigMutator(fn) Option` | 创建前修改 `openai.ClientConfig` |

## 使用

```go
package example

import (
	"context"

	aiModel "github.com/liujitcn/kratos-kit/ai/model"
	openai "github.com/sashabaranov/go-openai"
)

func Example(ctx context.Context) error {
	cfg := &aiModel.ModelConfig{
		Provider:  aiModel.ProviderOpenAICompatible,
		ModelName: "gpt-4o",
		APIKey:    "sk-xxx",
		BaseURL:   "https://api.openai.com/v1",
	}

	client, err := aiModel.NewClient(cfg)
	if err != nil {
		return err
	}

	_, err = client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: cfg.ModelName,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: "你好"},
		},
	})
	return err
}
```
