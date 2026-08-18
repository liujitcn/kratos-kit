# server 模块说明

`server` 只提供服务端入口，按协议拆分为四个独立 Go 模块：

- `server/http`：Kratos HTTP 服务端与 HTTP 服务端中间件。
- `server/grpc`：Kratos gRPC 服务端与 gRPC 服务端拦截器。
- `server/mcp`：MCP 独立服务和可挂载 Handler。
- `server/sse`：SSE 独立服务和可挂载 Handler。

客户端连接、客户端重试、客户端熔断和客户端请求标识不属于本目录，由上层应用的 client 模块负责。

## HTTP

导入别名示例：

```go
serverhttp "github.com/liujitcn/kratos-kit/server/http"
```

服务端入口：

```go
func CreateHttpServer(cfg *configv1.Bootstrap, mds ...middleware.Middleware) (*kratosHttp.Server, error)
```

`CreateHttpServer` 从 `cfg.server.http` 读取监听地址、超时、CORS、TLS 以及 recovery、tracing、ratelimit、metadata 等官方 Kratos 中间件开关。传输层中间件固定在调用方传入的 `mds` 之前；参数校验由 `kratos-core` 按 `middleware.enable_validate` 挂载。

HTTP 服务端专属中间件位于 `server/http/middleware`：

- `requestid`：读取或生成 `X-Request-Id`，并写入响应头和上下文。
- `metrics`：记录请求数、耗时和进行中请求数。
- `timeout`：限制服务端 Handler 的执行时间。

HTTP 服务端配置示例：

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
```

## gRPC

导入别名示例：

```go
servergrpc "github.com/liujitcn/kratos-kit/server/grpc"
```

服务端入口：

```go
func CreateGrpcServer(cfg *configv1.Bootstrap, mds ...middleware.Middleware) (*kratosGrpc.Server, error)
```

`CreateGrpcServer` 从 `cfg.server.grpc` 读取 network、addr、timeout、TLS、健康检查以及 recovery、tracing、ratelimit、metadata 等官方 Kratos 中间件开关。传输层中间件固定在调用方传入的 `mds` 之前；参数校验由 `kratos-core` 按 `middleware.enable_validate` 挂载。

gRPC 服务端专属拦截器位于 `server/grpc/middleware`：

- `requestid`：读取或生成 `X-Request-Id` metadata，并写入响应 metadata 和上下文。
- `metrics`：记录一元和流式 RPC 指标。
- `timeout`：为没有 deadline 的服务端 RPC 增加默认超时。

gRPC 服务端配置示例：

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

`enable_validate` 控制 `kratos-core` 的业务校验；独立使用 `server/http` 或 `server/grpc` 时，请通过 `mds` 显式传入校验中间件。

## MCP

导入别名示例：

```go
servermcp "github.com/liujitcn/kratos-kit/server/mcp"
```

服务端入口：

```go
func CreateMcpServer(cfg *configv1.Bootstrap, opts ...mcpServer.ServerOption) (*mcpServer.Server, error)
func CreateMcpSSEServer(cfg *configv1.Bootstrap, opts ...mcpServer.ServerOption) (*mcpServer.Server, error)
func CreateMcpHandler(cfg *configv1.Bootstrap, opts ...mcpServer.ServerOption) (*mcpServer.Server, error)
func CreateMcpHTTPHandler(cfg *configv1.Bootstrap, opts ...mcpServer.ServerOption) (http.Handler, error)
func CreateMcpSSEHandler(cfg *configv1.Bootstrap, opts ...mcpServer.ServerOption) (http.Handler, error)
func WithMcpServerOptions(opts ...func(*mcp.ServerOptions)) mcpServer.ServerOption
```

`CreateMcpServer` 创建独立 Streamable HTTP MCP 服务，`CreateMcpSSEServer` 创建 Legacy SSE MCP 服务；`CreateMcpHandler` 创建进程内服务，可通过两个 Handler 方法挂载到 HTTP 宿主。MCP 客户端不在本模块提供。

## SSE

导入别名示例：

```go
serversse "github.com/liujitcn/kratos-kit/server/sse"
```

服务端入口：

```go
func CreateSseServer(cfg *configv1.Bootstrap, opts ...sseServer.ServerOption) (*sseServer.Server, error)
func CreateSseHandler(cfg *configv1.Bootstrap, opts ...sseServer.ServerOption) (*sseServer.Server, error)
func CreateSseHTTPHandler(cfg *configv1.Bootstrap, opts ...sseServer.ServerOption) (http.Handler, error)
```

`CreateSseServer` 创建独立 SSE 服务，`CreateSseHandler` 和 `CreateSseHTTPHandler` 用于挂载到已有 HTTP 服务。SSE 客户端不在本模块提供。

## 中间件归属

官方 Kratos 中间件直接使用，不在 `server` 中复制：`recovery`、`metadata`、`ratelimit`、`logging` 和 `circuitbreaker`。参数校验属于 `kratos-core` 的业务中间件，`server/*` 不自动挂载；`circuitbreaker` 是客户端能力，不应装配到服务端。

`kratos-core/server/middleware` 负责认证、鉴权、业务日志、国际化和业务校验等应用策略；它依赖 `server/http`、`server/grpc` 和官方 Kratos，但 `server/*` 不反向依赖 core。

推荐的服务端链路顺序为：

```text
request-id -> timeout -> recovery -> tracing -> metadata -> rate-limit -> i18n -> logging -> authn/authz -> validate -> handler
```

`server` 是服务端传输适配的 seam，`core` 负责业务中间件组合；同一能力只允许一个层级拥有实现，避免官方中间件、kit 中间件和 core 中间件重复执行。
