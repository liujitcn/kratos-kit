# ai/model 模块说明

`ai/model` 基于 `github.com/sashabaranov/go-openai` 封装 OpenAI 兼容客户端创建能力。

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
      organization: org_xxx
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
	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	openai "github.com/sashabaranov/go-openai"
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

	client, err := aiModel.NewClient(cfg)
	if err != nil {
		return err
	}

	_, err = client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: cfg.GetModelName(),
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: "你好"},
		},
	})
	return err
}
```
