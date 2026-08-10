# rpc 包说明

`rpc` 包提供 Kratos HTTP、gRPC、MCP、SSE 服务端、Handler 与客户端的统一构建入口，并内置常用中间件（recovery、tracing、validate、ratelimit、metadata）的配置化装配能力。

当前目录核心文件：

- `rpc/http.go`：HTTP 服务端构建
- `rpc/grpc.go`：gRPC 服务端与 gRPC 客户端构建
- `rpc/mcp.go`：MCP 独立服务、HTTP Handler 与客户端构建
- `rpc/sse.go`：SSE 独立服务、HTTP Handler 与客户端构建
- `rpc/middleware/validate`：基于 `protovalidate` 的请求校验中间件
- `rpc/middleware/requestid`：请求 ID 注入中间件

## HTTP

### 服务端方法

```go
func CreateHttpServer(cfg *configv1.Bootstrap, mds ...middleware.Middleware) (*kratosHttp.Server, error)
```

- 从 `cfg.server.http` 读取配置（监听地址、超时、CORS、TLS、中间件开关）。
- `mds ...middleware.Middleware` 会追加到内置中间件之后。
- 当 `cfg.server.http.enable_pprof=true` 时，会自动注册 pprof 路由。

配置示例：

```yaml
server:
  http:
    network: tcp
    addr: 0.0.0.0:8000
    timeout: 5s
    enable_pprof: false
    cors:
      headers: ["Content-Type", "Authorization"]
      methods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
      origins: ["*"]
    middleware:
      enable_recovery: true
      enable_tracing: true
      enable_validate: true
      enable_metadata: true
      limiter:
        name: bbr
    tls:
      file:
        cert_path: /path/to/server.crt
        key_path: /path/to/server.key
```

使用示例：

```go
httpSrv, err := rpc.CreateHttpServer(cfg)
if err != nil {
    return err
}
_ = httpSrv
```

### 客户端方法

```go
func CreateHttpClient(ctx context.Context, r registry.Discovery, serviceName string, cfg *configv1.Bootstrap, mds ...middleware.Middleware) (*kratosHttp.Client, error)
```

- 自动注入服务发现：`kratosHttp.WithDiscovery(r)`。
- 当 `serviceName` 不以 `discovery:///` 开头时，会自动补齐前缀。
- 从 `cfg.client.http` 读取 timeout、TLS 与中间件配置。

配置示例：

```yaml
client:
  http:
    timeout: 5s
    middleware:
      enable_recovery: true
      enable_tracing: true
      enable_metadata: true
```

使用示例：

```go
httpClient, err := rpc.CreateHttpClient(ctx, discovery, "user.service", cfg)
if err != nil {
    return err
}
_ = httpClient
```

## gRPC

### 服务端方法

```go
func CreateGrpcServer(cfg *configv1.Bootstrap, mds ...middleware.Middleware) (*kratosGrpc.Server, error)
```

- 从 `cfg.server.grpc` 读取配置（network、addr、timeout、TLS、中间件开关）。
- `mds ...middleware.Middleware` 会追加到内置中间件之后。

配置示例：

```yaml
server:
  grpc:
    network: tcp
    addr: 0.0.0.0:9000
    timeout: 5s
    middleware:
      enable_recovery: true
      enable_tracing: true
      enable_validate: true
      enable_metadata: true
      limiter:
        name: bbr
```

使用示例：

```go
grpcSrv, err := rpc.CreateGrpcServer(cfg)
if err != nil {
    return err
}
_ = grpcSrv
```

### 客户端方法

```go
func CreateGrpcClient(ctx context.Context, r registry.Discovery, serviceName string, cfg *configv1.Bootstrap, mds ...middleware.Middleware) (grpc.ClientConnInterface, error)
```

- `serviceName` 非空时保持服务发现语义；不带协议时自动补齐 `discovery:///`。
- `serviceName` 为空时读取 `cfg.client.grpc.endpoint`；不带协议时自动补齐 `direct:///`。
- `discovery:///` 地址必须传入 `registry.Discovery`，直连地址不依赖注册中心。
- 从 `cfg.client.grpc` 读取 timeout、TLS 与中间件配置。
- 通过 `kratosGrpc.DialInsecure` 建连；若配置了 TLS，会在 option 中设置 `WithTLSConfig`。

配置示例：

```yaml
client:
  grpc:
    endpoint: 127.0.0.1:9000
    timeout: 5s
    middleware:
      enable_recovery: true
      enable_tracing: true
      enable_metadata: true
```

使用示例：

```go
conn, err := rpc.CreateGrpcClient(ctx, discovery, "user.service", cfg)
if err != nil {
    return err
}
_ = conn
```

直连模式可以将 `serviceName` 和 `discovery` 设为空，由配置提供地址：

```go
conn, err := rpc.CreateGrpcClient(ctx, nil, "", cfg)
if err != nil {
    return err
}
_ = conn
```

## MCP

MCP 配置通过 `server.mcp` 与 `client.mcp` 配套使用。`server.mcp.transport` 可选 `UNSPECIFIED`、`HTTP`、`SSE`、`STDIO`、`IN_PROCESS`；`client.mcp.transport` 可选 `UNSPECIFIED`、`HTTP`、`SSE`、`STDIO`。`UNSPECIFIED` 默认按 `HTTP` 处理，`IN_PROCESS` 只用于服务端，客户端按实际暴露协议选择 `HTTP` 或 `SSE`。

### 方法

```go
func CreateMcpServer(cfg *configv1.Bootstrap, opts ...mcpServer.ServerOption) (*mcpServer.Server, error)
func CreateMcpSSEServer(cfg *configv1.Bootstrap, opts ...mcpServer.ServerOption) (*mcpServer.Server, error)
func CreateMcpHandler(cfg *configv1.Bootstrap, opts ...mcpServer.ServerOption) (*mcpServer.Server, error)
func CreateMcpHTTPHandler(cfg *configv1.Bootstrap, opts ...mcpServer.ServerOption) (http.Handler, error)
func CreateMcpSSEHandler(cfg *configv1.Bootstrap, opts ...mcpServer.ServerOption) (http.Handler, error)
func CreateMcpClient(ctx context.Context, cfg *configv1.Bootstrap, opts ...func(*mcp.ClientOptions)) (*mcp.ClientSession, error)
func WithMcpServerOptions(opts ...func(*mcp.ServerOptions)) mcpServer.ServerOption
```

- `CreateMcpServer` 创建独立服务，默认是 Streamable HTTP，也可按 `server.mcp.transport=STDIO` 创建 stdio 服务端。
- `CreateMcpSSEServer` 创建独立监听的 Legacy SSE MCP 服务。
- `CreateMcpHandler` 创建 in-process MCP 服务，可通过 `HTTPHandler()` 或 `SSEHandler()` 挂载到已有 HTTP 服务。
- `CreateMcpHTTPHandler` 与 `CreateMcpSSEHandler` 是直接返回 `http.Handler` 的便捷方法。
- `CreateMcpClient` 固定使用客户端标识 `Name: "mcp-client"`、`Version: "1.0.0"`，从 `cfg.client.mcp` 读取 transport、endpoint、headers、timeout、stdio command/args/env/work_dir。
- `WithMcpServerOptions` 用于透传官方 MCP SDK 服务端选项。工具、资源、prompt、middleware 和 session 能力均基于官方 SDK；底层服务可通过返回实例的 `MCPServer()` 获取，typed 工具推荐使用 `mcp.AddTool[In, Out]`。

### 模式速查

| 模式 | 服务端方法 | 服务端配置 | 客户端方法 | 客户端配置 |
| --- | --- | --- | --- | --- |
| 独立 Streamable HTTP | `CreateMcpServer` | `server.mcp.transport=HTTP` 或 `UNSPECIFIED` | `CreateMcpClient` | `client.mcp.transport=HTTP` 或 `UNSPECIFIED`，配置 `http.endpoint` |
| 独立 Legacy SSE | `CreateMcpSSEServer` | `server.mcp.transport=SSE` | `CreateMcpClient` | `client.mcp.transport=SSE`，配置 `sse.endpoint` |
| In-process HTTP 挂载 | `CreateMcpHandler` 后调用 `HTTPHandler()`，或直接 `CreateMcpHTTPHandler` | `server.mcp.transport=IN_PROCESS` | `CreateMcpClient` | `client.mcp.transport=HTTP`，endpoint 为外部 HTTP 服务挂载路径 |
| In-process SSE 挂载 | `CreateMcpHandler` 后调用 `SSEHandler()`，或直接 `CreateMcpSSEHandler` | `server.mcp.transport=IN_PROCESS` | `CreateMcpClient` | `client.mcp.transport=SSE`，endpoint 为外部 HTTP 服务挂载路径 |
| stdio 子进程 | 子进程内调用 `CreateMcpServer` | `server.mcp.transport=STDIO` | `CreateMcpClient` | `client.mcp.transport=STDIO`，配置 `stdio.command/args/env/work_dir` |

### 独立 Streamable HTTP

```yaml
server:
  mcp:
    transport: HTTP
    network: tcp
    addr: 127.0.0.1:7003
    path: /mcp
    enable_keepalive: false
    streamable_http:
      stateless: true
      json_response: true

client:
  mcp:
    transport: HTTP
    http:
      endpoint: http://127.0.0.1:7003/mcp
      timeout: 10s
      headers:
        X-App: demo
```

```go
srv, err := rpc.CreateMcpServer(cfg)
if err != nil {
    return err
}
session, err := rpc.CreateMcpClient(ctx, clientCfg)
if err != nil {
    return err
}
_ = srv
_ = session
```

### 独立 Legacy SSE

Legacy SSE 是 MCP 的 SSE transport，不等同于下面普通 `server.sse` 事件服务。

```yaml
server:
  mcp:
    transport: SSE
    network: tcp
    addr: 127.0.0.1:7004
    path: /sse
    enable_keepalive: false
    legacy_sse:
      disable_localhost_protection: false

client:
  mcp:
    transport: SSE
    sse:
      endpoint: http://127.0.0.1:7004/sse
      timeout: 10s
      headers:
        X-App: demo
```

```go
srv, err := rpc.CreateMcpSSEServer(cfg)
if err != nil {
    return err
}
session, err := rpc.CreateMcpClient(ctx, clientCfg)
if err != nil {
    return err
}
_ = srv
_ = session
```

### In-process HTTP 挂载

```yaml
server:
  mcp:
    transport: IN_PROCESS
    path: /mcp
    streamable_http:
      stateless: true
      json_response: true

client:
  mcp:
    transport: HTTP
    http:
      endpoint: http://127.0.0.1:8000/mcp
      timeout: 10s
```

```go
mcpSrv, err := rpc.CreateMcpHandler(cfg)
if err != nil {
    return err
}
handler, err := mcpSrv.HTTPHandler()
if err != nil {
    return err
}
httpSrv.Handle("/mcp", handler)

session, err := rpc.CreateMcpClient(ctx, clientCfg)
if err != nil {
    return err
}
_ = session
```

### In-process SSE 挂载

```yaml
server:
  mcp:
    transport: IN_PROCESS
    path: /sse
    legacy_sse:
      disable_localhost_protection: false

client:
  mcp:
    transport: SSE
    sse:
      endpoint: http://127.0.0.1:8000/sse
      timeout: 10s
```

```go
mcpSrv, err := rpc.CreateMcpHandler(cfg)
if err != nil {
    return err
}
handler, err := mcpSrv.SSEHandler()
if err != nil {
    return err
}
httpSrv.Handle("/sse", handler)

session, err := rpc.CreateMcpClient(ctx, clientCfg)
if err != nil {
    return err
}
_ = session
```

### stdio 子进程

服务端配置写在被客户端启动的命令行进程中；客户端通过 `stdio.command` 启动该进程，没有 HTTP endpoint。

```yaml
server:
  mcp:
    transport: STDIO
    enable_keepalive: false

client:
  mcp:
    transport: STDIO
    stdio:
      command: go
      args:
        - run
        - ./cmd/mcpstdio
      env:
        APP_ENV: dev
      work_dir: /path/to/project
```

服务端命令：

```go
srv, err := rpc.CreateMcpServer(serverCfg)
if err != nil {
    return err
}
return srv.Start(ctx)
```

客户端：

```go
session, err := rpc.CreateMcpClient(ctx, clientCfg)
if err != nil {
    return err
}
_ = session
```

### HTTP Tool 配置

`server.mcp.http_tools` 可以把普通 HTTP、Gin、Kratos HTTP 或第三方 HTTP 接口暴露为 MCP Tool。该配置跟随 MCP 服务端，无论 MCP 是独立 HTTP、独立 SSE，还是 in-process 挂载，都会在创建服务时注册。

```yaml
server:
  mcp:
    transport: IN_PROCESS
    path: /mcp
    streamable_http:
      stateless: true
      json_response: true
    http_tools:
      - name: get_user_profile
        description: 获取用户资料
        method: POST
        base_url: http://127.0.0.1:8101
        url: /users/{id}/profile
        headers:
          X-From: mcp
        body_mode: HTTP_BODY_MODE_JSON
        timeout: 2s
        parameters:
          - name: id
            location: HTTP_PARAM_LOCATION_PATH
            required: true
            type: string
          - name: verbose
            location: HTTP_PARAM_LOCATION_QUERY
            type: boolean
          - name: trace
            target: X-Trace
            location: HTTP_PARAM_LOCATION_HEADER
            required: true
            type: string
          - name: name
            location: HTTP_PARAM_LOCATION_BODY
            required: true
            type: string
```

## SSE

这里的 SSE 是普通事件推送服务，配置为 `server.sse` 与 `client.sse`；不要和 MCP 的 `client.mcp.sse` 混用。

### 方法

```go
func CreateSseServer(cfg *configv1.Bootstrap, opts ...sse.ServerOption) (*sse.Server, error)
func CreateSseHandler(cfg *configv1.Bootstrap, opts ...sse.ServerOption) (*sse.Server, error)
func CreateSseHTTPHandler(cfg *configv1.Bootstrap, opts ...sse.ServerOption) (http.Handler, error)
func CreateSseClient(endpoint string, opts ...sse.ClientOption) *sse.Client
func CreateSseClientWithConfig(cfg *configv1.Bootstrap, opts ...sse.ClientOption) (*sse.Client, error)
```

- `CreateSseServer` 从 `cfg.server.sse` 创建独立监听端口的 SSE 服务。
- `CreateSseHandler` 创建可挂载到已有 HTTP 服务的 SSE 处理器，不会单独监听端口。
- `CreateSseHTTPHandler` 返回标准 `http.Handler`，便于挂载到 Kratos HTTP Server 或原生 `net/http`。
- `CreateSseClientWithConfig` 复用 `cfg.client.sse.endpoint`、timeout、metadata、encode_base64 与 TLS 配置创建客户端。

### 模式速查

| 模式 | 服务端方法 | 服务端配置 | 客户端方法 | 客户端配置 |
| --- | --- | --- | --- | --- |
| 独立 SSE 服务 | `CreateSseServer` | `server.sse.transport=HTTP` 或 `UNSPECIFIED` | `CreateSseClientWithConfig` | `client.sse.endpoint=http://host:port/events` |
| In-process 挂载 | `CreateSseHandler` 或 `CreateSseHTTPHandler` | `server.sse.transport=IN_PROCESS` | `CreateSseClientWithConfig` | endpoint 为外部 HTTP 服务挂载路径 |

### 独立 SSE 服务

```yaml
server:
  sse:
    transport: HTTP
    network: tcp
    addr: 127.0.0.1:7002
    path: /events
    codec: json
    timeout: 10s
    event_ttl: 300s
    auto_stream: true
    auto_reply: true
    split_data: true
    encode_base64: false

client:
  sse:
    endpoint: http://127.0.0.1:7002/events
    timeout: 10s
    metadata:
      X-App: demo
    encode_base64: false
```

```go
srv, err := rpc.CreateSseServer(cfg)
if err != nil {
    return err
}
client, err := rpc.CreateSseClientWithConfig(clientCfg)
if err != nil {
    return err
}
_ = srv
_ = client
```

### In-process 挂载

```yaml
server:
  sse:
    transport: IN_PROCESS
    path: /events
    codec: json
    auto_stream: true
    auto_reply: true

client:
  sse:
    endpoint: http://127.0.0.1:8000/events
    timeout: 10s
```

```go
sseSrv, err := rpc.CreateSseHandler(cfg)
if err != nil {
    return err
}
httpSrv.Handle("/events", sseSrv)

client, err := rpc.CreateSseClientWithConfig(clientCfg)
if err != nil {
    return err
}
_ = client
```

## 内置中间件装配规则

HTTP 与 gRPC 服务端支持以下开关（位于 `configv1.Server.Middleware`）：

- `enable_recovery` -> `recovery.Recovery()`
- `enable_tracing` -> `tracing.Server()`
- `enable_validate` -> `validate.ProtoValidate()`
- `enable_metadata` -> `metadata.Server()`
- `limiter.name == "bbr"` -> `ratelimit`（BBR）

HTTP 与 gRPC 客户端支持以下开关（位于 `configv1.Client.Middleware`）：

- `enable_recovery` -> `recovery.Recovery()`
- `enable_tracing` -> `tracing.Client()`
- `enable_metadata` -> `metadata.Client()`
- `enable_circuit_breaker` -> Kratos v3 `circuitbreaker.Client()`

Kratos v3 的熔断中间件用于客户端下游调用，不装配在 HTTP 或 gRPC 服务端。

`server.grpc.custom_health` 为 `false` 时由 Kratos 注册并维护标准 gRPC
health service；设为 `true` 时调用 `grpc.CustomHealth()`，由业务自行注册
健康服务。

## validate 中间件

`rpc` 不再维护重复的 validate 中间件。服务端开启 `enable_validate` 时，
`rpc/server_middleware.go` 会使用 Kratos v3 的统一入口：

```go
validate.Validator(func(value any) error {
    message, ok := value.(proto.Message)
    if !ok {
        return nil
    }
    return protovalidate.Validate(message)
})
```

校验逻辑：

- 若请求实现 `proto.Message`，使用 `protovalidate.Validate` 校验。
- Kratos `middleware/validate.Validator` 同时兼容旧式 `Validate() error` 接口。
- Kratos 统一把校验错误转换为 `errors.BadRequest("VALIDATOR", err.Error())` 并保留 cause。

## requestid 中间件

`rpc/middleware/requestid` 提供：

- `Server(opts ...RequestIDOption) middleware.Middleware`
- `Client(opts ...RequestIDOption) middleware.Middleware`
- `WithRequestID(ctx context.Context, requestID string) context.Context`
- `FromContext(ctx context.Context) string`
- `NewRequestIDMiddleware(opts ...RequestIDOption) middleware.Middleware`
- `GetRequestID(ctx context.Context) string`
- `WithRequestIDHeader(name string)`
- `WithRequestIDGenerator(f func() string)`

`NewRequestIDMiddleware` 和 `GetRequestID` 仅保留旧调用兼容；新代码使用
`Server` / `Client` 与 `FromContext`。

行为说明：

- 若上下文中已有 request id，则直接透传。
- 否则默认生成 UUID 并写入上下文。

## pprof 路由（HTTP）

当 `enable_pprof=true` 时，`CreateHttpServer` 会注册：

- `/debug/pprof`
- `/debug/cmdline`
- `/debug/profile`
- `/debug/symbol`
- `/debug/trace`
- `/debug/allocs`
- `/debug/block`
- `/debug/goroutine`
- `/debug/heap`
- `/debug/mutex`
- `/debug/threadcreate`

## 测试

在 `rpc` 目录执行：

```bash
cd rpc
go test ./...
```
