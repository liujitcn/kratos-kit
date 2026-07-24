# MCP

在大语言模型（LLM）时代，模型与外部系统之间的连接方式正在经历深刻变革。Model Context
Protocol（MCP）正是在这一背景下应运而生的开放通信协议，旨在通过标准化手段，构建起 LLM 与外部数据资源、可调用工具之间的桥梁，从而让
AI 应用更易构建、更强交互，彻底改变 AI 工具的连接方式以及它们与现实世界的交互范式。

MCP（模型上下文协议）是一种标准化的通信协议，专为 AI 工具（如聊天机器人、代码助手、AI Agent 等）与外部系统的集成而设计。它为 AI
引入了“使用工具”的能力框架，使其不仅能理解自然语言，还能主动调用系统资源、访问数据或执行操作。MCP Server 的出现，正在重塑 AI
的能力边界，使其从单纯的对话机器演化为能够完成实际任务的智能助手。在 MCP 出现之前，开发者若希望让 AI 工具访问 Gmail、Google
Drive 或天气 API 等外部系统，通常需要为每个集成单独编写定制逻辑，硬编码对每一个 API 的连接方式。这不仅增加了开发成本，也阻碍了
AI 工具的可扩展性与通用性。而 MCP 的核心价值，在于为模型与系统之间的通信建立统一标准，降低接入门槛、提升开发效率。它正如 HTTP
之于 Web 应用，是支撑智能系统互联互通的基础协议之一。

简单来说，MCP 把过去“一个模型对一个系统”的烟囱式集成，变成了“多模型对多能力”的标准化连接网络。这是推动 AI 工具平台化和生态化的关键一步。

## 服务创建方式

`transport/mcp` 支持四种运行形态：

- `ServerTypeHTTP`：独立监听端口的 Streamable HTTP MCP 服务。
- `ServerTypeSSE`：独立监听端口的 Legacy SSE MCP 服务。
- `ServerTypeInProcess`：进程内 MCP 服务，可通过 `HTTPHandler()` 或 `SSEHandler()` 挂载到外部 HTTP 服务。
- `ServerTypeStdio`：基于标准输入输出的 MCP 服务。

独立 Streamable HTTP 服务示例：

```go
srv := mcp.NewServer(
	mcp.WithServerType(mcp.ServerTypeHTTP),
	mcp.WithListenAddress(":7003"),
	mcp.WithHandlerPath("/mcp"),
)
```

上述方式会创建独立的 Streamable HTTP MCP 服务，调用 `Start` 后监听指定端口，并在 `/mcp` 路径提供 MCP 处理器。

独立 Legacy SSE 服务示例：

```go
srv := mcp.NewServer(
	mcp.WithServerType(mcp.ServerTypeSSE),
	mcp.WithListenAddress(":7004"),
	mcp.WithHandlerPath("/sse"),
)
```

`HTTP` 与 `SSE` 都是独立 HTTP 服务形态，`Endpoint()` 返回自身 HTTP 地址；它们不需要 MCP 额外启动 keepalive。

进程内挂载示例：

```go
srv := mcp.NewHandler()
handler, err := srv.HTTPHandler()
if err != nil {
	return err
}
httpServer.Handle("/mcp", handler)
```

上述方式等同 `ServerTypeInProcess`，不会单独监听端口，也不会单独启动 keepalive；它依附于外部 HTTP 服务，服务注册与保活应由外部 HTTP 服务处理。`HTTPHandler` 会返回标准 `http.Handler`，可挂载到 Kratos HTTP Server 或原生 `net/http`。如果需要挂载 Legacy SSE，可以使用 `srv.SSEHandler()`。

`ServerTypeStdio` 没有 HTTP 监听端口，默认会创建 keepalive 端点用于服务注册；如果只在本地命令行场景使用，可以通过 `mcp.WithEnableKeepAlive(false)` 关闭。
