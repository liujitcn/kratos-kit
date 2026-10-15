# kratos-kit

`kratos-kit` 是面向 Kratos v3 应用的 Go 多模块工具库。仓库内的模块围绕启动、配置、传输、数据访问、安全、消息、观测、AI 和工作流提供可复用能力；每个模块都可以按自己的 module path 单独引入。

## 模块地图

### 启动与基础设施

| 模块 | 能力 |
| --- | --- |
| [`api`](api/README.md) | Buf 管理的 protobuf 配置契约和生成代码。 |
| [`bootstrap`](bootstrap/README.md) | 应用信息、配置解密、配置加载、日志、注册、追踪和 Kratos App 启动。 |
| [`config`](config/README.md) | 本地文件、HTTP、Redis、Vault、ZooKeeper、对象存储以及配置中心 Source。 |
| [`key`](key/README.md) | 根密钥读取、派生和 File、Vault、AWS、Google、Azure、Kubernetes Provider。 |
| [`logger`](logger/README.md) | 标准日志、Zap、Logrus、Fluent、云日志和 Zerolog 工厂。 |
| [`registry`](registry/README.md) | 注册与发现工厂，以及 Consul、Etcd、Eureka、Kubernetes、Nacos、Polaris、ServiceComb、ZooKeeper 实现。 |
| [`tracer`](tracer/README.md) | OpenTelemetry TracerProvider 和 Zipkin、OTLP、stdout exporter。 |
| [`tracing`](tracing/README.md) | Kratos HTTP、gRPC、broker 的 OpenTelemetry 中间件和传播工具。 |
| [`runtime`](runtime/README.md) | 进程内共享数据库、缓存、队列、对象存储、锁、翻译器和密钥实例。 |
| [`sdk`](sdk/README.md) | 以 `sdk.Runtime` 暴露共享运行时。 |
| [`utils`](utils/README.md) | TLS、Redis 连接选项和 RPC Tool 名称转换等工具。 |

### 服务、传输与消息

| 模块 | 能力 |
| --- | --- |
| [`server`](server/README.md) | HTTP、gRPC、MCP、SSE 服务端构造和配置。 |
| [`transport`](transport/README.md) | Kratos transport 辅助、SSE、MCP、队列、Cron、高精度定时器和保活服务。 |
| [`broker`](broker/README.md) | Broker 接口、发布订阅、请求响应、typed handler 和 Kratos 生命周期适配。 |
| [`broker/nats`](broker/nats/README.md) | Core NATS、JetStream、队列订阅、请求响应和消息追踪。 |
| [`queue`](queue/README.md) | 内存队列、Redis Stream 队列和延迟消息。 |
| [`cache`](cache/README.md) | 内存、Redis、Redis Cluster 缓存和原子限流状态存储。 |
| [`locker`](locker/README.md) | 进程内锁和 Redis 分布式锁。 |
| [`health`](health/README.md) | 并发健康检查聚合和 liveness/readiness HTTP handler。 |
| [`metrics`](metrics/README.md) | Prometheus、OpenTelemetry OTLP、Datadog 指标接口和 Provider。 |
| [`retry`](retry/README.md) | context 感知的重试、退避和抖动。 |
| [`ratelimit`](ratelimit/README.md) | Sentinel 和令牌桶限流器接口，供 Kratos middleware 组合使用。 |
| [`encoding/*`](encoding/README.md) | Avro、BSON、CBOR、FlatBuffers、Gob、Thrift、TOML codec。 |
| [`swagger-ui`](swagger-ui/README.md) | 嵌入式 Swagger UI、ReDoc 和受保护的 OpenAPI 文档路由。 |
| [`pprof`](pprof/README.md) | Pyroscope 性能采样生命周期封装。 |

### 数据、安全与外部服务

| 模块 | 能力 |
| --- | --- |
| [`database/gorm`](database/gorm/README.md) | GORM 多驱动客户端、迁移、审计、租户/数据范围/项目隔离和可观测性。 |
| [`database/ent`](database/ent/README.md) | Ent driver、连接池、迁移回调、表注释和审计 mixin。 |
| [`auth`](auth/README.md) | 认证、JWT/Session/API Key/Basic/HMAC/mTLS/OAuth2/OIDC 以及 Casbin/OPA/Cerbos/Zanzibar 鉴权适配。 |
| [`oauth`](oauth/README.md) | GitHub、Gitee、Google、微信、企业微信、钉钉、飞书 OAuth 流程。 |
| [`notify`](notify/README.md) | SMTP、Webhook、钉钉、飞书、微信、企业微信和短信 Sender。 |
| [`oss`](oss/README.md) | 本地、FTP、MinIO、阿里云 OSS、AWS S3 及兼容对象存储。 |
| [`captcha`](captcha/README.md) | 数字、字符串、中文、算术验证码，以及滑动、点击、旋转行为验证码。 |
| [`redact`](redact/README.md) | Proto 字段运行时脱敏、流式响应包装和存储策略。 |
| [`secretcrypto`](secretcrypto/README.md) | 一次性 RSA 公钥、AES-GCM 字段加密和请求中间件。 |
| [`translator`](translator/README.md) | Google、百度、阿里云、火山引擎机器翻译统一接口。 |

### AI 与工作流

| 模块 | 能力 |
| --- | --- |
| [`ai/model`](ai/model/README.md) | OpenAI 兼容模型客户端。 |
| [`ai/eino`](ai/eino/README.md) | Eino Chat/Responses 模型、Chain、Graph、Workflow、Tool Node。 |
| [`ai/langchaingo`](ai/langchaingo/README.md) | LangChainGo LLM、Embedding、VectorStore 和 Agent。 |
| [`workflow`](workflow/README.md) | Argo、Conductor、go-workflows、Temporal 的公共 Client/Worker 接口。 |
| [`workflow/argo`](workflow/argo/README.md) | Argo Server Workflow 提交、查询、日志和生命周期操作。 |
| [`workflow/conductor`](workflow/conductor/README.md) | Conductor 工作流执行和 Task Worker。 |
| [`workflow/goworkflows`](workflow/goworkflows/README.md) | 进程内持久工作流 Client 和 Worker。 |
| [`workflow/temporal`](workflow/temporal/README.md) | Temporal 工作流、Activity、Signal、Query 和 Worker。 |

### 命令行工具

| 命令 | 用途 |
| --- | --- |
| [`cmd/normalize-go-imports`](cmd/normalize-go-imports/README.md) | 按目录名统一 Go import 别名，支持预览和写回。 |
| [`cmd/protoc-gen-go-agent-tool`](cmd/protoc-gen-go-agent-tool/README.md) | 从 protobuf service 生成 Eino Agent Tool。 |
| [`cmd/protoc-gen-go-mcp-tool`](cmd/protoc-gen-go-mcp-tool/README.md) | 从 protobuf service 生成 MCP Tool 注册代码。 |
| [`cmd/protoc-gen-go-redact`](cmd/protoc-gen-go-redact/README.md) | 从 protobuf 字段规则生成脱敏代码和规则契约。 |
| [`cmd/kratos-admin`](cmd/kratos-admin/README.md) | 按项目名生成后端模板、目录结构和三端前端初始化结果。 |

## 安装

根目录是多模块工作区，使用具体模块路径安装。例如：

```bash
go get github.com/liujitcn/kratos-kit/bootstrap@latest
go get github.com/liujitcn/kratos-kit/config@latest
go get github.com/liujitcn/kratos-kit/server/http@latest
go get github.com/liujitcn/kratos-kit/database/gorm@latest
go get github.com/liujitcn/kratos-kit/auth@latest
```

按需追加实现模块，例如 `config/etcd`、`logger/zap`、`registry/nacos`、`database/gorm/driver/mysql`、`metrics/prometheus`、`workflow/temporal`。

## 最小启动示例

实现包通过空白导入注册工厂，应用使用 `bootstrap.RunApp` 统一启动：

```go
package main

import (
	"github.com/go-kratos/kratos/v3"
	"github.com/liujitcn/kratos-kit/bootstrap"

	_ "github.com/liujitcn/kratos-kit/config/etcd"
	_ "github.com/liujitcn/kratos-kit/logger/zap"
	_ "github.com/liujitcn/kratos-kit/registry/etcd"
)

func initApp(ctx *bootstrap.Context) (*kratos.App, func(), error) {
	return bootstrap.NewApp(ctx), func() {}, nil
}

func main() {
	ctx := bootstrap.NewContext(nil, nil)
	if err := bootstrap.RunApp(ctx, initApp); err != nil {
		panic(err)
	}
}
```

`bootstrap` 支持本地配置、远程配置、密钥派生、日志、注册发现、链路追踪和应用信息初始化。根密钥配置使用独立的 `key.yaml`，敏感值可使用 `ENC[...]` 标记加密。

## 配置与边界

- `api/proto/config/v1` 是配置契约唯一来源，`api/gen/go` 只能由 `buf generate` 生成。
- `database/gorm` 支持 `database` 和 `databases` 两种固定数据源配置；跨数据源事务、Join 和请求级动态切库不属于该模块职责。
- Redis Stream 队列生产者默认按 `MAXLEN` 裁剪；消息消费成功后执行确认并删除，关闭时会停止拉取并等待已拉取消息处理。
- 验证码统一由 `Generate` 生成并写入缓存，由 `Verify` 校验，成功后删除缓存；图片只以 base64 或 JSON base64 返回。
- `oauth` 只负责授权地址、state/PKCE、换取 Token 和用户信息，不负责业务用户、登录态或权限。
- `secretcrypto` 的临时私钥只在缓存中短期保存；同一请求的多个密文应复用同一 `key_id`。

## 生成与开发

```bash
make plugin  # 安装 protobuf、Kratos、脱敏、Agent Tool 和 MCP Tool 插件
make api     # 在 api 目录执行 buf generate
make gen     # 生成 API 并整理 Go import
make fmt     # 规范化 import 别名并运行 goimports
```

直接安装仓库内命令：

```bash
go install ./cmd/normalize-go-imports
go install ./cmd/protoc-gen-go-agent-tool
go install ./cmd/protoc-gen-go-mcp-tool
go install ./cmd/protoc-gen-go-redact
go install ./cmd/kratos-admin
```

`kratos-admin create <name>` 拒绝覆盖已有目录，默认生成 `system` 业务模块；可通过 `--modules` 传入以逗号分隔的模块名，通过 `--module` 指定后端 Go module 路径。生成过程会创建目录、渲染模板、初始化三端前端并执行后端验证。

## 验证

修改 Go 代码后，在受影响的 module 目录执行：

```bash
go test ./...
go vet ./...
```

修改 protobuf 时，在 `api` 目录执行 `buf lint`、`buf build`、`buf generate` 和 `go test ./...`。修改文档不需要生成代码，但新增或移动文档后应检查相对链接和代码示例。

## 文档索引

各模块 README 是对应 API 和行为的详细说明。命令模板文档位于 `cmd/kratos-admin/templates`；`cmd/protoc-gen-go-redact` 另提供中文、英文和日文说明。

## 许可证

代码许可证见 [LICENSE](LICENSE)。依赖库的许可证和版权声明以各自发行版本为准。
