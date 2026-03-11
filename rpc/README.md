# rpc 包说明

`rpc` 包提供 Kratos HTTP/gRPC 服务端与 gRPC 客户端的统一构建入口，并内置常用中间件（recovery、tracing、validate、ratelimit、metadata）的配置化装配能力。

当前目录核心文件：

- `rpc/http.go`：HTTP 服务端构建
- `rpc/grpc.go`：gRPC 服务端与 gRPC 客户端构建
- `rpc/middleware/validate`：基于 `protovalidate` 的请求校验中间件
- `rpc/middleware/requestid`：请求 ID 注入中间件

## 导出 API

### HTTP 服务端

```go
func CreateHttpServer(cfg *conf.Bootstrap, mds ...middleware.Middleware) (*kratosHttp.Server, error)
```

行为说明：

- 从 `cfg.server.http` 读取配置（监听地址、超时、CORS、TLS、中间件开关）。
- `mds ...middleware.Middleware` 会追加到内置中间件之后。
- 当 `cfg.server.http.enable_pprof=true` 时，会自动注册 pprof 路由。

### gRPC 服务端

```go
func CreateGrpcServer(cfg *conf.Bootstrap, mds ...middleware.Middleware) (*kratosGrpc.Server, error)
```

行为说明：

- 从 `cfg.server.grpc` 读取配置（network、addr、timeout、TLS、中间件开关）。
- `mds ...middleware.Middleware` 会追加到内置中间件之后。

### gRPC 客户端

```go
func CreateGrpcClient(ctx context.Context, r registry.Discovery, serviceName string, cfg *conf.Bootstrap, mds ...middleware.Middleware) (grpc.ClientConnInterface, error)
```

行为说明：

- 自动注入服务发现：`kratosGrpc.WithDiscovery(r)`。
- 当 `serviceName` 不以 `discovery:///` 开头时，会自动补齐前缀。
- 从 `cfg.client.grpc` 读取 timeout/TLS/中间件配置。
- `initGrpcClientConfig` 会返回更新后的 `options`，调用方会接收并继续建连（即内部追加 `WithTimeout/WithMiddleware/WithTLSConfig/WithNodeFilter` 均会生效）。
- 通过 `kratosGrpc.DialInsecure` 建连；若配置了 TLS，会在 option 中设置 `WithTLSConfig`。

### HTTP 客户端

```go
func CreateHttpClient(ctx context.Context, r registry.Discovery, serviceName string, cfg *conf.Bootstrap, mds ...middleware.Middleware) (*kratosHttp.Client, error)
```

行为说明：

- 自动注入服务发现：`kratosHttp.WithDiscovery(r)`。
- 当 `serviceName` 不以 `discovery:///` 开头时，会自动补齐前缀。
- 从 `cfg.client.http` 读取 timeout/TLS/中间件配置。
- `initHttpClientConfig` 会返回更新后的 `options`，调用方会接收并继续建连（即内部追加 `WithTimeout/WithMiddleware/WithTLSConfig/WithNodeFilter` 均会生效）。

## 内置中间件装配规则

HTTP 与 gRPC 服务端支持以下开关（位于 `conf.Server.Middleware`）：

- `enable_recovery` -> `recovery.Recovery()`
- `enable_tracing` -> `tracing.Server()`
- `enable_validate` -> `validate.ProtoValidate()`
- `enable_metadata` -> `metadata.Server()`
- `limiter.name == "bbr"` -> `ratelimit`（BBR）

gRPC 客户端支持以下开关（位于 `conf.Client.Middleware`）：

- `enable_recovery` -> `recovery.Recovery()`
- `enable_tracing` -> `tracing.Client()`
- `enable_metadata` -> `metadata.Client()`

说明：`enable_circuit_breaker` 字段目前在 `rpc` 包中尚未落地具体实现。

## validate 中间件

`rpc/middleware/validate` 提供：

```go
func ProtoValidate() middleware.Middleware
```

校验逻辑：

- 若请求实现 `proto.Message`，使用 `protovalidate.Validate` 校验。
- 同时兼容旧式 `Validate() error` 接口。
- 校验失败返回 `errors.BadRequest("VALIDATOR", err.Error())`。

## requestid 中间件

`rpc/middleware/requestid` 提供：

- `NewRequestIDMiddleware(opts ...RequestIDOption) middleware.Middleware`
- `GetRequestID(ctx context.Context) string`
- `WithRequestIDHeader(name string)`
- `WithRequestIDGenerator(f func() string)`

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

## 配置示例

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

client:
  grpc:
    timeout: 5s
    middleware:
      enable_recovery: true
      enable_tracing: true
      enable_metadata: true
```

## 最小使用示例

```go
httpSrv, err := rpc.CreateHttpServer(cfg)
if err != nil {
    return err
}

grpcSrv, err := rpc.CreateGrpcServer(cfg)
if err != nil {
    return err
}

conn, err := rpc.CreateGrpcClient(ctx, discovery, "user.service", cfg)
if err != nil {
    return err
}
_ = conn
_ = httpSrv
_ = grpcSrv
```

## 测试

在 `rpc` 目录执行：

```bash
cd rpc
go test ./...
```
