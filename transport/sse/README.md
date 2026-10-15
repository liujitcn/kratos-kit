# transport/sse

`transport/sse` 提供可接入 Kratos 生命周期的 SSE 服务端，以及独立的 SSE 客户端和事件流工具。服务端可以独立监听端口，也可以作为 `http.Handler` 挂载到已有 HTTP 服务。

## 服务端

独立监听端口：

```go
srv, err := sse.NewServer(
	sse.WithAddress(":7002"),
	sse.WithPath("/events"),
	sse.WithAutoStream(true),
)
if err != nil {
	return err
}
app := kratos.New(kratos.Server(srv))
```

挂载到已有 HTTP 服务：

```go
handler := sse.NewHandler(
	sse.WithPath("/events"),
	sse.WithAutoStream(true),
)
http.Handle("/events", handler)
```

`NewHandler` 不创建监听器。`NewServer` 实现 `transport.Server` 和 `transport.Endpointer`，`Start` 注册路由并开始服务，`Stop` 清理连接和已注册 stream。

## Stream

服务端可通过 `RegisterStream` 注册带 ID 的业务流定义，或使用自动 stream 模式。默认从 query 参数 `stream` 读取 stream ID，也可以通过 `WithStreamIDResolver` 自定义解析：

```go
handler := sse.NewHandler(
	sse.WithPath("/events/{stream}"),
	sse.WithStreamIDResolver(sse.ResolveStreamIDFromPathPrefix("/events/")),
	sse.WithAutoStream(true),
)
```

已经完成鉴权或 stream 映射时，可直接调用 `ServeStreamHTTP(w, r, streamID)`。服务端支持事件 TTL、缓冲区大小、自动重放、自动回复、数据拆分、Base64 编码、CORS、Header、token 提取和授权回调。

## SSE 格式

响应使用 `text/event-stream`。每个事件以空行分隔，支持 `id`、`event`、`data` 和 `retry` 字段；多行 data 会按 SSE 规则拼接。客户端可使用 `Last-Event-ID` 请求断线后的事件恢复。

```text
id: 42
event: order.updated
data: {"id":"order-1"}

```

## 客户端与工具

客户端支持请求超时、Header、TLS、事件解码和断线重连；`StreamManager` 负责服务端连接、订阅和事件缓存。事件日志可用于按 ID 重放，具体选项见 `client_options.go` 和 `server_options.go`。

## 约束

- SSE 是单向事件通道，客户端向服务端提交命令应使用普通 HTTP 或其他 RPC。
- 独立服务的 path 应与业务路由隔离；进程内 Handler 不会自动创建保活服务。
- 使用自动重放时应设置合理的事件 TTL 和缓冲区，避免无界缓存。
