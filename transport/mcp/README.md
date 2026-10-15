# MCP

`transport/mcp` 将官方 MCP SDK 适配为 Kratos transport，提供 Streamable HTTP、Legacy SSE、STDIO 和进程内四种服务形态，并可选配 keepalive 服务。

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
