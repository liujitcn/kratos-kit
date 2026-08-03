# api 模块说明

## 概述

`api` 模块用于维护 protobuf 定义并生成 Go 代码。

- proto 源码目录：`api/proto`
- 生成代码目录：`api/gen/go`（当前配置生成到 `api/gen/go/config/v1`）
- buf 配置：`api/buf.yaml`、`api/buf.gen.yaml`、`api/buf.lock`

## 目录结构

```text
api/
├── proto/              # proto 定义
│   └── config/v1/
├── gen/go/             # 生成结果（*.pb.go）
├── buf.yaml            # buf 模块与依赖配置
├── buf.gen.yaml        # 代码生成插件配置
└── buf.lock            # 依赖锁文件（自动生成）
```

## 依赖与工具

在仓库根目录执行：

```bash
make plugin
make cli
```

至少需要以下命令可用：

- `buf`
- `protoc-gen-go`
- `protoc-gen-go-grpc`
- `protoc-gen-go-http`
- `protoc-gen-go-errors`

## 生成代码

在仓库根目录执行：

```bash
make api
```

等价命令：

```bash
cd api
buf generate
```

`buf.gen.yaml` 已配置：

- `managed.enabled: true`
- `go_package_prefix: github.com/liujitcn/kratos-kit/api/gen/go`
- 输出路径：`gen/go`
- 生成插件：`go`、`go-grpc`、`go-http`、`go-errors`

## 更新 `buf.lock`

当你修改了 `api/buf.yaml` 中的 `deps`，或希望升级远程依赖版本时，执行：

```bash
cd api
buf dep update
```

说明：

- `buf.lock` 由 buf 自动维护，不要手动编辑。
- 更新后请一并提交 `buf.lock`。

## 校验建议

生成前后建议执行：

```bash
cd api
buf lint
buf build
buf generate
```

回到仓库根目录执行：

```bash
go test ./...
```

## Proto 引用约定

`buf` 模块根是 `api/proto`，因此在 proto 中引用同模块文件时，按模块内相对路径写 import，例如：

```proto
import "config/v1/tls.proto";
```

## 配置包约定

当前配置契约使用 `config.v1` 包，生成 Go 包路径为：

```go
import configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
```

启动配置 `configv1.Bootstrap` 当前包含 `server`、`client`、`data`、`trace`、`logger`、`registry`、`config`、`oss`、`notify`、`authn`、`authz`、`pprof`、`ai`、`oauth` 与 `translator`。AI 配置使用 `configv1.AI`，其中 `AI.Model` 与上游 `kratos-bootstrap` 的模型配置保持一致；旧的 `Client.Llm` 配置已移除。OAuth 配置使用独立的 `configv1.OAuth`，由 `oauth.NewManager` 直接按 `providers` 配置创建 Provider。翻译配置使用 `configv1.Translator`，由 `translator` 模块按 `type` 选择具体厂商。

`configv1.Data` 保留单数据库字段 `database`，并新增 `databases` map 用于按名称配置多个固定数据库；`Data.Database` 新增 `connection_timeout`，用于启动期连接校验。

服务端配置 `configv1.Server` 当前包含 `http`、`grpc`、`mcp` 与 `sse` 子配置；其中 `mcp` 用于描述新版 MCP 运行时配置，可表达 `HTTP`、`SSE`、`STDIO`、`IN_PROCESS` 四种运行形态、监听地址、挂载路径、TLS、优雅关闭超时、keepalive、Streamable HTTP 与 Legacy SSE Handler 选项，也可通过 `http_tools` 将 Gin、`net/http` 或其他 HTTP 接口配置为 MCP Tool；`sse` 用于描述通用 SSE 服务端的监听地址、路由路径、编解码器、TLS、超时、事件 TTL 与自动流/自动回复等开关。

## 发布引用约定

`api` 子模块承载公共 proto 契约和生成代码。配置契约或下游依赖基线同步发布时，外部项目应优先锁定最新 `api/v*` tag，确保 `config/v1` 包路径、`SSE` 配置字段与生成代码保持一致。
