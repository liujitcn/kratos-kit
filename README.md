# kratos-kit

`kratos-kit` 是一个基于 Kratos 的工具库集合，提供应用引导、配置加载、日志、注册发现、链路追踪，以及缓存/队列/鉴权/OSS/数据库等通用能力。

## 仓库说明

该仓库是多模块（multi-module）结构，根目录与子目录都包含 `go.mod`。常用模块包括：

当前依赖基线为 `github.com/go-kratos/kratos/v3 v3.0.0`，日志接口已迁移到 Kratos v3 的 `log/slog` 体系。

- `api`：protobuf 定义与代码生成（`buf generate`）
- `bootstrap`：应用启动入口（配置加载 + 日志 + 注册中心 + tracer + `kratos.App`）
- `config`：本地/远程配置加载与工厂注册；额外提供直接实现 Kratos `config.Source` 的文件、HTTP、Redis、Vault、ZooKeeper 和 S3 配置源
- `logger`：日志工厂（`std`/`zap`/`logrus`/`fluent`/`aliyun`/`tencent`/`zerolog`）
- `registry`：注册发现工厂（`consul`/`etcd`/`eureka`/`kubernetes`/`nacos`/`polaris`/`servicecomb`/`zookeeper`）
- `tracer`：OpenTelemetry TracerProvider 与 exporter 工厂（`std`/`zipkin`/`otlp-http`/`otlp-grpc`）
- `tracing`：OpenTelemetry 追踪适配层
- `ai`：AI 客户端与编排封装（含 `model`、`eino`、`langchaingo` 子模块）
- `auth`：认证与鉴权封装；认证支持 API Key、Basic、HMAC、JWT、mTLS、OAuth2、OIDC、Session，鉴权支持 Casbin、OPA、Cerbos 和 Zanzibar 适配端口
- `oauth`：第三方 OAuth SDK 封装（直接使用 `api` 下 OAuth 配置，支持 GitHub、Gitee、Google、微信开放平台、微信公众号、微信小程序、企业微信、钉钉、飞书；闭环支持 state、PKCE、授权地址、code 换 token、用户信息，不包含业务登录态）
- `cache`：内存/Redis 缓存封装
- `circuitbreaker`：Kratos 客户端请求级熔断适配及 Hystrix、Vegas、Sentinel 实现
- `queue`：内存/Redis 队列封装
- `locker`：Redis 分布式锁封装
- `oss`：本地/FTP/MinIO/阿里云 OSS/AWS S3 及兼容对象存储封装
- `translator`：基于配置的统一机器翻译封装，内置 Google/百度/阿里云/火山引擎
- [`database/gorm`](database/gorm/README.md)：GORM 客户端封装，提供多数据库 driver、连接池、迁移与可观测性，并内置审计字段填充、租户隔离和角色数据范围过滤；版本化迁移仅在脚本全部成功后记录，失败会记录错误并阻止应用启动
- `database/ent`：Ent 底层数据库 driver 封装（含 `mysql`/`postgres`/`sqlite` driver 子模块，支持连接池配置、debug SQL 日志、迁移回调、表/字段注释与审计字段 mixin）
- `broker`：消息发布订阅与 typed handler 封装，通用 `TransportServer` 可把任意 broker 接入 Kratos 应用生命周期；`broker/nats` 通过共享实现支持 Core NATS、JetStream、队列订阅、请求响应和消息追踪
- `workflow`：工作流引擎封装（含 `argo`、`conductor`、`goworkflows`、`temporal` 子模块），公共包只定义跨引擎一致的 `Client`/`Worker` 生命周期接口
- `transport`：通用传输辅助（含 `keepalive`、`mcp`、`sse` 子模块）
- `server/http`、`server/grpc`、`server/mcp`、`server/sse`：HTTP、gRPC、MCP、SSE 服务端配置封装，详细说明见 [server/README.md](server/README.md)
- `encoding`：直接适配 Kratos 的额外 codec（`avro`/`bson`/`cbor`/`flatbuffers`/`gob`/`thrift`/`toml`）；`msgpack`/`xml`/`yaml` 使用 Kratos v3 自带实现
- `health`：应用级 readiness 检查聚合与 HTTP handler
- `metrics`：Prometheus、OpenTelemetry OTLP、Datadog 指标适配
- `retry`：支持 context、退避和抖动的通用重试
- `ratelimit`：可注入 Kratos 服务端中间件的 Sentinel 和令牌桶限流器
- `swagger-ui`：Swagger UI 嵌入与路由注册封装（支持 `net/http` 与 Kratos）
- `pprof`：性能采样封装（当前支持 `pyroscope`）
- `captcha`：验证码生成与存储封装
- `sdk`：共享运行时入口，统一保存数据库、缓存、队列、OSS、锁和翻译器实例
- `runtime`：运行时应用信息模型
- `utils`：通用工具（TLS、Redis 配置辅助）
- `cmd/project-docs`：收集当前项目约定 README 和根 docs 的 Go 命令
- `cmd/kratos-admin-backend`：生成基于 kratos-admin Core 的空业务后端项目

`database/gorm` 的 `Data` 配置支持 `database` 与 `databases` 两种形式。多个固定数据源应按名称分别创建客户端和 `data.Data`，每个客户端启动时主动校验连接；跨数据源事务、Join 与请求级动态切库不在该封装的职责范围内。

其中 `queue` 模块内的 Redis Stream 实现默认会为生产者开启基于 `MAXLEN` 的长度裁剪；消费者在消费成功后会执行 `XACK` 并立即 `XDEL` 删除消息实体，保持“消费即删除”的队列语义。消费者执行 `Shutdown` 时会停止继续拉取新消息，并尽量处理完本地已拉取但尚未确认的消息后再退出。生产者和消费者同时提供 `EnqueueContext`、`RunContext`、`RegisterWithLastIDContext`，便于调用方按业务请求或后台任务生命周期控制 Redis 操作。

`captcha` 模块提供普通图形验证码和行为验证码两类能力：普通图形验证码支持数字、字符串、中文、算术；行为验证码支持滑动拼图、点击文字、旋转图片。所有验证码统一通过 `Generate(ctx)` 生成并自动写入 `cache.Cache`，通过 `Verify(ctx, id, input)` 校验并在成功后删除缓存。验证码图片只以 base64 或 JSON(base64) 返回给前端，不落盘、不对外返回答案。

`workflow` 模块按工作流引擎拆分为独立 Go 模块：`workflow/argo` 通过 Argo Server REST API 操作 Kubernetes Workflow；`workflow/conductor` 封装 Conductor SDK 和任务 Worker；`workflow/goworkflows` 提供进程内持久工作流客户端与 Worker；`workflow/temporal` 封装 Temporal 客户端、Worker、Signal、Query 和默认消息工作流。

## 安装

请按模块路径安装，而不是安装根模块。例如：

```bash
go get github.com/liujitcn/kratos-kit/bootstrap@latest
go get github.com/liujitcn/kratos-kit/config@latest
go get github.com/liujitcn/kratos-kit/logger@latest
go get github.com/liujitcn/kratos-kit/registry@latest
go get github.com/liujitcn/kratos-kit/tracer@latest
go get github.com/liujitcn/kratos-kit/transport/mcp@latest
go get github.com/liujitcn/kratos-kit/transport/sse@latest
go get github.com/liujitcn/kratos-kit/server/http@latest
go get github.com/liujitcn/kratos-kit/server/grpc@latest
go get github.com/liujitcn/kratos-kit/server/mcp@latest
go get github.com/liujitcn/kratos-kit/server/sse@latest
go get github.com/liujitcn/kratos-kit/config/redis@latest
go get github.com/liujitcn/kratos-kit/config/vault@latest
go get github.com/liujitcn/kratos-kit/config/zookeeper@latest
go get github.com/liujitcn/kratos-kit/config/oss@latest
go get github.com/liujitcn/kratos-kit/encoding/avro@latest
go get github.com/liujitcn/kratos-kit/encoding/bson@latest
go get github.com/liujitcn/kratos-kit/encoding/cbor@latest
go get github.com/liujitcn/kratos-kit/encoding/flatbuffers@latest
go get github.com/liujitcn/kratos-kit/encoding/thrift@latest
go get github.com/liujitcn/kratos-kit/health@latest
go get github.com/liujitcn/kratos-kit/metrics/prometheus@latest
go get github.com/liujitcn/kratos-kit/retry@latest
go get github.com/liujitcn/kratos-kit/ratelimit/sentinel@latest
go get github.com/liujitcn/kratos-kit/ratelimit/tokenbucket@latest
go get github.com/liujitcn/kratos-kit/oss/s3@latest
go get github.com/liujitcn/kratos-kit/auth/authn/engine/apikey@latest
go get github.com/liujitcn/kratos-kit/auth/authn/engine/oidc@latest
go get github.com/liujitcn/kratos-kit/auth/authz/engine/opa@latest
go get github.com/liujitcn/kratos-kit/auth/authz/engine/cerbos@latest
go get github.com/liujitcn/kratos-kit/broker/nats@latest
go get github.com/liujitcn/kratos-kit/swagger-ui@latest
go get github.com/liujitcn/kratos-kit/pprof@latest
go get github.com/liujitcn/kratos-kit/oauth@latest
go get github.com/liujitcn/kratos-kit/translator@latest
go get github.com/liujitcn/kratos-kit/ai/model@latest
go get github.com/liujitcn/kratos-kit/ai/eino@latest
go get github.com/liujitcn/kratos-kit/ai/langchaingo@latest
go get github.com/liujitcn/kratos-kit/database/gorm@latest
go get github.com/liujitcn/kratos-kit/database/gorm/driver/mysql@latest
go get github.com/liujitcn/kratos-kit/database/ent@latest
go get github.com/liujitcn/kratos-kit/database/ent/driver/mysql@latest
go get github.com/liujitcn/kratos-kit/workflow@latest
go get github.com/liujitcn/kratos-kit/workflow/argo@latest
go get github.com/liujitcn/kratos-kit/workflow/conductor@latest
go get github.com/liujitcn/kratos-kit/workflow/goworkflows@latest
go get github.com/liujitcn/kratos-kit/workflow/temporal@latest
```

项目文档收集命令使用 `go install` 安装：

```bash
go install github.com/liujitcn/kratos-kit/cmd/project-docs@latest
```

命令从项目根目录扫描相对路径不超过三段的文件，只收集精确命名的
`README.md`，以及任意 `docs` 目录中的 Markdown。普通项目默认输出到
`internal/projectdocs`；包含 `backend` 的仓库默认输出到 `backend/internal/docs`。
同一路径可以通过文件名语言后缀提供翻译，例如 `README.en-US.md` 或
`docs/guide.zh-TW.md`；语言版本会聚合到同一个文档节点，并以
`locale` 字段输出。无后缀文件仍作为默认正文。收集命令只使用显式存在的语言
Markdown，并在源文未变化时保留上一次生成的 `locale` 内容；不执行自动翻译。
也可以通过 `--output` 或 `-o` 指定生成目录：

```bash
project-docs
project-docs --output ./backend/internal/docs
```

生成物不包含项目身份。服务加载后使用 `AppInfo.Project` 和 `AppInfo.Name`
生成稳定文档 ID，并与 OpenAPI/Swagger 保持一致。

后端项目生成命令使用 Go module 末段作为目标目录名，生成 Proto、biz、service、
server、data、migration 和 docs 分层骨架，并同时提供可挂载模块与独立应用
两套 Wire 装配入口：

```bash
go install github.com/liujitcn/kratos-kit/cmd/kratos-admin-backend@latest
kratos-admin-backend create --module github.com/example/order
```

## 快速开始

### 1. 引入需要的实现包（通过 `init` 自动注册）

```go
import (
	_ "github.com/liujitcn/kratos-kit/config/etcd"
	_ "github.com/liujitcn/kratos-kit/logger/zap"
	_ "github.com/liujitcn/kratos-kit/registry/etcd"
)
```

### 2. 启动应用

```go
package main

import (
	"github.com/go-kratos/kratos/v3"
	"github.com/liujitcn/kratos-kit/bootstrap"
)

func initApp(ctx *bootstrap.Context) (*kratos.App, func(), error) {
	app := bootstrap.NewApp(ctx)
	return app, func() {}, nil
}

func main() {
	ctx := bootstrap.NewContext(nil, nil)
	if err := bootstrap.RunApp(ctx, initApp); err != nil {
		panic(err)
	}
}
```

### 3. 创建 Ent 数据库客户端

`database/ent` 返回通用 `entgo.io/ent/dialect.Driver` 包装，业务项目需要用自己生成的 Ent client 接入：

```go
package data

import (
	"context"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	entkit "github.com/liujitcn/kratos-kit/database/ent"
	_ "github.com/liujitcn/kratos-kit/database/ent/driver/mysql"

	"your-app/ent"
)

func NewEntClient(cfg *configv1.Data_Database) (*ent.Client, func(), error) {
	drv, cleanup, err := entkit.NewEntClient(cfg)
	if err != nil {
		return nil, nil, err
	}

	client := ent.NewClient(ent.Driver(drv))
	if cfg.GetEnableMigrate() {
		if err = client.Schema.Create(context.Background()); err != nil {
			cleanup()
			return nil, nil, err
		}
		if err = drv.RunRegisteredTableComments(context.Background()); err != nil {
			cleanup()
			return nil, nil, err
		}
	}
	return client, cleanup, nil
}
```

Ent 审计字段可在 schema 中复用 `entkit.AuditMixin{}`；字段注释通过 `entsql.WithComments(true)` 落库，表注释可用 `entkit.RegisterTableComment("users", "用户表")` 在迁移后回填。Doris 可通过 MySQL 协议访问，导入 `database/ent/driver/mysql` 后配置 `driver: doris` 即可；Doris 建表、分区、分桶等能力建议使用专用 SQL 管理，不建议依赖 Ent 自动迁移。

默认命令行参数（`bootstrap/flag.go`）：

- `-c, --conf`：配置目录，默认 `../../configs`
- `-e, --env`：运行环境，默认 `dev`
- `-s, --chost`：配置中心地址，默认 `127.0.0.1:8500`
- `-t, --ctype`：配置中心类型，默认 `consul`
- `-d, --daemon`：以守护进程方式运行（非 Windows）
- `-p, --project`：覆盖 app-info 的项目标识
- `-a, --app-id`：覆盖 app-info 的应用标识
- `-i, --instance-id`：覆盖 app-info 的实例标识
- `-n, --name`：覆盖 app-info 的应用名称
- `-v, --version`：覆盖 app-info 的应用版本

app-info 字段的优先级为：启动参数 → 传入的 `*configv1.AppInfo` → 默认值。

## 配置加载行为

`config.LoadBootstrapConfig(configPath)` 的行为：

1. 始终加载本地配置源（`configPath`）。
2. 若存在 `${configPath}/config.yaml`，先读取其中 `config.type`，再创建对应远程配置源并合并加载。
3. 扫描 `configv1.Bootstrap` 及已注册的自定义配置结构。

引导配置工厂支持的远程配置源类型由 `config.type` 决定，可选值见
`config/types.go`：`apollo`/`consul`/`etcd`/`kubernetes`/`nacos`/`polaris`。
独立模块 `config/fs`、`config/http`、`config/redis`、`config/vault`、
`config/zookeeper`、`config/oss` 可直接作为 Kratos `config.Source` 使用。

## AI 配置

AI 相关配置位于 `bootstrap.ai`，其中 `ai.model` 参照上游 `kratos-bootstrap` 的模型配置结构。旧的 `client.llm` 配置已移除。

云端 OpenAI 兼容 API：

```yaml
ai:
  model:
    type: CLOUD_MODEL
    model_name: gpt-4o
    temperature: 0.7
    max_tokens: 4096
    timeout_seconds: 60
    max_retries: 3
    cloud:
      api_key: sk-xxx
      base_url: https://api.openai.com/v1
      organization: org_xxx
```

本地 Ollama：

```yaml
ai:
  model:
    type: LOCAL_MODEL
    model_name: llama3
    timeout_seconds: 120
    local:
      host: 127.0.0.1
      port: 11434
      use_gpu: true
```

## API 代码生成

```bash
make api
```

`buf` 模块根目录为 `api/proto`，同模块 proto 引用使用模块内路径，例如：

```proto
import "config/v1/tls.proto";
```

### Agent 与 MCP Tool 生成

`protoc-gen-go-agent-tool` 用于从 proto service 生成 Eino Agent Tool 封装，生成文件后缀为 `_agent_tool.go`。生成后的构造函数返回 `tool.InvokableTool`，接收 `XxxServiceServer`，直接调用本进程内的服务端方法，不经过 gRPC client 或本地网络转换。

`protoc-gen-go-mcp-tool` 用于从 proto service 生成 MCP Tool 注册代码，生成文件后缀为 `_mcp_tool.go`。Agent 与 MCP 生成器都基于标准 gRPC 方法路径生成 Tool 名称，例如 `/admin.v1.AuthService/GetUserInfo` 会生成 `admin_v1_auth_service_get_user_info`。外部如需复用同一转换规则，可调用 `utils.ToolNameFromRPCPath`。

当请求或响应消息存在递归字段引用时，Agent 与 MCP 生成器会将 Tool 的 schema 泛型降级为 `any`，再在执行时转换为真实 proto 请求，避免 JSON Schema 生成阶段出现递归循环错误。

```bash
protoc --go-agent-tool_out=. path/to/service.proto
protoc --go-mcp-tool_out=. path/to/service.proto
```

## 开发命令

```bash
make plugin   # 安装 protoc 相关插件
make cli      # 安装 kratos/buf 等命令行工具
make fmt      # 使用 goimports 统一整理 Go 代码
make api      # 生成 api 代码
make gen      # 一键生成并整理 api 代码
make tag      # 默认从仓库根目录递归检查 go.mod 并自动打/推送 tag（含根模块）
make tag MODULE=auth       # 从 auth 目录开始递归检查 go.mod 并打 tag
make tag MODULE=auth/authn # 从 auth/authn 目录开始递归检查 go.mod 并打 tag
```

依赖版本基线更新时，需要同步整理各子模块的 `go.mod`/`go.sum`，并逐模块执行 `go test -mod=readonly ./...`。`api` 子模块承载公共 proto 契约和生成代码，配置契约或依赖基线同步发布时，需要确认新的 `api/v*` tag 跟随发布，避免下游引用到旧生成代码。

## 子模块文档

- [api/README.md](api/README.md)
- [ai/model/README.md](ai/model/README.md)
- [ai/eino/README.md](ai/eino/README.md)
- [ai/langchaingo/README.md](ai/langchaingo/README.md)
- [bootstrap/README.md](bootstrap/README.md)
- [captcha/README.md](captcha/README.md)
- [config/README.md](config/README.md)
- [database/gorm/README.md](database/gorm/README.md)
- [database/ent/README.md](database/ent/README.md)
- [logger/README.md](logger/README.md)
- [registry/README.md](registry/README.md)
- [tracer/README.md](tracer/README.md)
- [server/README.md](server/README.md)
- [swagger-ui/README.md](swagger-ui/README.md)
- [workflow/README.md](workflow/README.md)
- [workflow/argo/README.md](workflow/argo/README.md)
- [workflow/conductor/README.md](workflow/conductor/README.md)
- [workflow/goworkflows/README.md](workflow/goworkflows/README.md)
- [workflow/temporal/README.md](workflow/temporal/README.md)
- [oauth/README.md](oauth/README.md)
- [auth/authn/README.md](auth/authn/README.md)
- [auth/authz/README.md](auth/authz/README.md)
- [auth/authn/engine/jwt/README.md](auth/authn/engine/jwt/README.md)
- [auth/authz/engine/casbin/README.md](auth/authz/engine/casbin/README.md)
- [transport/keepalive/README.md](transport/keepalive/README.md)
- [transport/mcp/README.md](transport/mcp/README.md)
- [transport/sse/README.md](transport/sse/README.md)

## 来源与版权说明

本仓库大部分代码参考或来源于以下开源项目，并在此基础上结合当前业务需求进行了整理与调整：

- [tx7do/kratos-bootstrap](https://github.com/tx7do/kratos-bootstrap)
- [tx7do/kratos-transport](https://github.com/tx7do/kratos-transport)
- [tx7do/kratos-authn](https://github.com/tx7do/kratos-authn)
- [tx7do/kratos-authz](https://github.com/tx7do/kratos-authz)

若涉及版权或授权边界问题，请优先以上游项目许可证与仓库声明为准，并联系维护者处理。
