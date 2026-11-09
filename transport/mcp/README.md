# MCP

`transport/mcp` 基于 `github.com/mark3labs/mcp-go v1.5.0` 适配 Kratos transport，提供 Streamable HTTP、Legacy SSE、STDIO 和进程内四种服务形态，并可选配 keepalive 服务。

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

## 原生选项与工具注册

底层实例由 `srv.MCPServer()` 返回，类型为 `*server.MCPServer`。导入 `github.com/mark3labs/mcp-go/server` 后，可直接传入原生函数选项：

```go
srv := mcp.NewHandler(
	mcp.WithServerOptions(server.WithInstructions("提供业务工具")),
	mcp.WithStreamableHTTPOptions(server.WithStateLess(true)),
)
```

`WithServerOptions`、`WithMCPServerOptions`、`WithStreamableHTTPOptions`、`WithSSEOptions` 以及两个 Handler 方法均接收对应的原生可变参数选项；多次调用按顺序追加，后面的选项优先。

`AddTool` 和 `RegisterHandler` 接收 `mcp-go/mcp.Tool` 值与 `server.ToolHandlerFunc`。处理函数接收值类型的 `mcp.CallToolRequest`，可通过 `request.BindArguments(&req)` 绑定业务参数，通过 `mcp.NewToolResultJSON(reply)` 同时返回文本和结构化结果。默认启用输入 schema 校验。

Legacy SSE 沿用同一个挂载路径处理 GET 连接和 POST 消息，客户端使用 endpoint 事件中的消息地址。独立服务由 `Stop` 释放连接和后台任务；持有进程内服务实例的调用方应在外部 HTTP 服务关闭时调用 `srv.Stop(ctx)`。

从 `modelcontextprotocol/go-sdk` 迁移时，需要同步调整原生选项、Tool 与处理函数类型，并重新运行 `protoc-gen-go-mcp-tool` 生成注册代码。现有服务类型与配置字段保持兼容；`json_response` 映射到 `WithJSONResponse`，只约束 POST 响应格式；无状态模式关闭 GET 事件流；`session_timeout` 映射到 `WithSessionIdleTTL`。
