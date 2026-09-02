# api 模块说明

## 模块定位

`api` 是独立的 Go 子模块（`github.com/liujitcn/kratos-kit/api`），负责维护公共
protobuf 契约并生成 Go 代码。`api/proto` 下的 proto 文件是唯一事实来源，
`api/gen/go` 下的文件均由 Buf 生成，不应手工修改。

当前模块使用 Go 1.27，配置契约包名为 `config.v1`。

## 目录结构

```text
api/
├── proto/config/v1/    # protobuf 源码
├── gen/go/config/v1/   # 生成的 Go 文件（*.pb.go）
├── buf.yaml            # Buf 模块、依赖、lint 与 breaking 配置
├── buf.gen.yaml        # 代码生成配置
├── buf.lock            # Buf 依赖锁定文件（自动生成）
├── go.mod
└── go.sum
```

## Proto 契约

所有文件位于 `api/proto/config/v1`，生成后使用同一个 Go 包：

| 文件 | 主要消息 | 当前能力 |
| --- | --- | --- |
| `bootstrap.proto` | `Bootstrap` | 聚合服务端、客户端、数据、链路、日志、注册中心、配置中心、对象存储、通知、认证、授权、pprof、AI、OAuth、翻译和 MFA 配置。 |
| `app_info.proto` | `AppInfo`、`Endpoint` | 应用标识、实例、版本、主机、端点、环境、区域、标签和构建信息。 |
| `server.proto` | `Server` | HTTP、gRPC、MCP、SSE 服务端；包含中间件、TLS、CORS、Swagger、pprof、请求体限制和健康检查。 |
| `client.proto` | `Client` | HTTP、gRPC、MCP、SSE 客户端；包含 JWT、元数据、重试、令牌桶限流、Prometheus 指标和 TLS。 |
| `data.proto` | `Data` | 单数据库 `database`、命名数据库 `databases`、Redis 和内存/Redis 队列；数据库支持连接池、迁移、追踪、指标及连接超时。 |
| `config.proto` | `Config` | Etcd、Consul、Nacos、Apollo、Kubernetes、Polaris 配置中心。 |
| `registry.proto` | `Registry` | Consul、Etcd、ZooKeeper、Nacos、Kubernetes、Eureka、Polaris、ServiceComb 注册发现。 |
| `logger.proto` | `Logger` | Zap、Logrus、Fluent、阿里云、腾讯云和 Zerolog。 |
| `oss.proto` | `Oss` | FTP、阿里云 OSS、MinIO、AWS S3 及兼容对象存储；支持根目录和上传前安全扫描。 |
| `authn.proto` | `Authentication` | JWT 签名、访问/刷新令牌有效期、强制白名单、可选认证规则和服务端 Session 生命周期。 |
| `authz.proto` | `Authorization` | Casbin 策略前缀及按 URL/HTTP 方法排除授权检查。 |
| `mfa.proto` | `Mfa` | 加密密钥、登录/绑定挑战、失败次数、恢复码、TOTP 和 WebAuthn。 |
| `oauth.proto` | `OAuth`、`Provider` | 按 Provider 名称配置 OAuth Client ID、密钥、回调地址和 Scope。 |
| `translator.proto` | `Translator` | Google、百度、阿里云、火山引擎翻译 Provider，支持超时和扩展参数。 |
| `tracer.proto` | `Tracer`、`BatcherOptions` | 导出器、端点、采样率、连接安全、BatchSpanProcessor 和 TraceContext/Baggage。 |
| `pprof.proto` | `Pprof` | Pyroscope 上报地址、认证、上传频率、标签和性能数据类型。 |
| `ai.proto` | `AI`、`AI.Model` | 云端模型与 Ollama 本地模型，包含模型名、温度、最大 Token、超时和重试。 |
| `tls.proto` | `Tls` | 文件或内存证书、CA、域名和跳过服务端证书校验。 |
| `key.proto` | `Key` | File、Vault、AWS、Google、Azure、Kubernetes 密钥 Provider 的非敏感启动参数。 |
| `notify.proto` | `Notification` | 通知类型及短信 Provider 配置。 |

### Bootstrap 顶层字段

`bootstrap.proto` 中的 `Bootstrap` 只聚合应用启动时需要的通用配置。除 `key.proto` 外，
下表中的字段均可作为 `Bootstrap` 的顶层字段：

| 字段 | 类型 | 用途 |
| --- | --- | --- |
| `server` | `Server` | HTTP、gRPC、MCP、SSE 服务端。 |
| `client` | `Client` | HTTP、gRPC、MCP、SSE 客户端及客户端中间件。 |
| `data` | `Data` | 数据库、Redis 和队列。 |
| `trace` | `Tracer` | 链路导出、采样和批处理。 |
| `logger` | `Logger` | Zap、Logrus、Fluent、云日志和 Zerolog。 |
| `registry` | `Registry` | 服务注册与发现。 |
| `config` | `Config` | 远程配置中心。 |
| `oss` | `Oss` | 对象存储和上传安全扫描。 |
| `notify` | `Notification` | 通知 Provider，目前提供短信配置。 |
| `authn` | `Authentication` | JWT 和服务端 Session 认证配置。 |
| `authz` | `Authorization` | Casbin 授权配置。 |
| `pprof` | `Pprof` | Pyroscope 性能数据上报。 |
| `ai` | `AI` | 云端或 Ollama 本地大模型。 |
| `oauth` | `OAuth` | 按名称配置第三方 OAuth Provider。 |
| `translator` | `Translator` | Google、百度、阿里云或火山引擎翻译。 |
| `mfa` | `Mfa` | MFA 加密、挑战、恢复码、TOTP 和 WebAuthn。 |

`app_info.proto` 中的 `AppInfo` 是独立的运行时元数据消息，不嵌入 `Bootstrap`。
`key.proto` 中的 `Key` 也是独立配置，直接对应 `key.yaml`，不嵌套在 `Bootstrap` 中。

### 配置存在性与兼容性

- `Bootstrap` 的顶层配置以及各 Provider 配置大多使用 `optional` 消息字段，应用可以区分
  “未配置”和“已配置但使用零值”。
- `map` 字段用于命名配置或扩展参数，例如 `Data.databases`、`OAuth.providers`、
  `Translator.options`；`repeated` 字段用于端点、Header、标签和规则列表。
- Proto 字段编号是兼容性契约。新增字段应使用未占用编号，不要修改或复用既有编号；
  `mfa.proto` 中的 `reserved` 字段和名称也不得重新使用。

### 服务端传输

`Server.Http` 可配置监听网络和地址、普通请求超时、请求体大小上限、CORS、服务端中间件、
TLS、Swagger UI 和 pprof。流式请求是否跳过普通请求超时由服务实现处理。

`Server.Grpc` 可配置监听网络和地址、请求超时、服务端中间件、TLS 以及自定义健康检查。

`Server.Mcp` 支持 `HTTP`、`SSE`、`STDIO`、`IN_PROCESS` 四种运行形态，可配置监听
网络/地址、挂载路径、TLS、优雅关闭超时和 keepalive。`streamable_http` 与
`legacy_sse` 分别描述两类 MCP Handler：前者支持无状态、JSON 响应、Session 超时和
localhost 保护；后者支持 localhost 保护开关。

`http_tools` 可把 Gin、`net/http` 或其他 HTTP 接口映射为 MCP Tool。每个 Tool 支持固定
Header；参数可写入路径、查询参数、Header 或 Body，并可声明描述、必填、Schema 类型、
目标名称和默认值。Body 支持 `NONE`、`JSON`、`FORM`、`RAW` 四种模式；`input_schema`
可直接提供 JSON Schema，未配置时可由参数定义推导。此外还可以配置 Body 模板和单次调用
超时。

`Server.Sse` 支持独立 HTTP 或进程内运行，可配置路由、编解码器、TLS、事件 TTL、
请求超时以及自动流、自动回复、数据拆分和 Base64 编码开关。

### 客户端传输与中间件

`Client.Mcp` 支持 Streamable HTTP、Legacy SSE 和 STDIO，分别通过 `http`、`sse`、
`stdio` 配置远程端点、Header、超时或子进程命令。STDIO 还支持命令参数、环境变量和
工作目录。

`Client.Http`、`Client.Grpc` 和 `Client.Sse` 分别支持端点、超时、TLS、请求元数据；
SSE 客户端额外支持事件内容 Base64 解码。

`Client.Middleware` 的 `auth`、`selector_filter`、`retry`、`rate_limiter` 和 `metrics`
字段在配置存在时启用对应能力。重试支持最大尝试次数、指数退避、总等待上限、幂等方法
前缀、重试状态码和排除方法；限流支持令牌速率、突发容量、等待模式和排除方法；指标
支持 namespace、subsystem、计数器/直方图/仪表盘名称和排除方法。

### 数据层与基础设施

`Data` 支持以下组合：

- `database` 配置默认数据库，`databases` 通过名称配置多个固定数据库；数据库支持驱动、
  DSN、调试、迁移、链路追踪、指标、连接池、连接最大生命周期、启动连接校验超时以及
  Prometheus Pushgateway 相关字段。
- `redis` 支持多个地址、密码、数据库索引、拨号/读写超时、TLS、链路追踪和指标。
- `queue.memory` 使用内存队列并配置工作池大小；`queue.redis` 使用 Redis Stream，
  可分别配置生产者的 Stream 长度策略和消费者的可见性超时、阻塞读取、消息回收、缓冲区
  大小及并发数。

`Config` 支持 Etcd、Consul、Nacos、Apollo、Kubernetes 和 Polaris。`Registry` 支持
Consul、Etcd、ZooKeeper、Nacos、Kubernetes、Eureka、Polaris 和 ServiceComb；每个
Provider 按自身协议提供端点、命名空间、认证、心跳、缓存或刷新参数。

`Logger` 支持 Zap、Logrus、Fluent、阿里云、腾讯云和 Zerolog。文件型日志可配置级别、
文件路径、大小、保留天数、备份数量和控制台输出；云日志 Provider 需要配置接入地址和
认证信息。

`Oss` 支持 FTP、阿里云 OSS、MinIO、AWS S3 及兼容服务，公共字段为 `type` 和
`root_directory`。`upload_security` 可在上传前执行外部扫描命令，命令参数由上传模块
固定追加。`Notification` 当前提供短信 Provider，可配置接入地址、地域和认证信息。

### 认证、安全与第三方登录

`Authentication` 同时支持 JWT 和服务端 Session：

- `jwt` 支持签名算法、密钥、访问/刷新令牌有效期，以及按 `prefix`、`regex`、`path`、
  `match` 匹配的强制白名单和可选认证规则。可选认证路由在携带令牌时仍会尝试解析用户。
- `session` 支持空闲超时和最大生命周期，零值表示使用应用默认值。

`Authorization.Casbin` 支持策略前缀和按 URL/HTTP 方法排除授权检查。`Tls` 支持从文件
或内存加载证书、私钥和 CA，并可设置域名及是否跳过服务端证书校验。

`Mfa` 支持 Base64 编码的 32 字节加密密钥、登录挑战有效期、绑定票据有效期、登录最大
失败次数、恢复码数量和长度，以及 TOTP 与 WebAuthn RP 配置。TOTP 支持发行方、周期、
时间窗口、密钥长度、位数和算法；WebAuthn 支持 RP ID 与允许的来源列表。敏感密钥不应
直接提交到 Proto 配置或仓库，应通过 Secret Manager、Kubernetes Secret、环境变量或工作
负载身份注入。

`OAuth.providers` 是以 Provider 名称为键的映射，Provider 配置包含 Client ID、Client
Secret、回调地址和 Scope。常见名称包括 `github`、`gitee`、`google`、`wechat`、
`wechatmp`、`wechatmini`、`wechatwork`、`dingtalk` 和 `feishu`。

`Key` 描述应用启动阶段的根密钥 Provider，支持 `file`、`vault`、`aws`、`google`、
`azure` 和 `kubernetes`。公共字段为 Provider 类型、派生范围、根密钥名称和版本；Provider
扩展字段分别为文件路径、Vault 地址/命名空间/value key、AWS 区域/版本阶段、Google
项目、Azure Vault 地址以及 Kubernetes 命名空间/value key。根密钥和认证凭据不属于该
Proto 的明文配置内容。

### 观测、运行时与扩展能力

`Tracer` 支持导出器、端点、采样率、环境、连接安全、BatchSpanProcessor 参数，以及
TraceContext 和 Baggage 传播开关。`Pprof` 当前支持 Pyroscope，可配置应用名、服务地址、
Basic Auth、租户、上传频率、标签、性能数据类型、GC 行为和自定义 HTTP Header。

`AI.Model` 支持 `CLOUD_MODEL` 和 `LOCAL_MODEL`：云端模型使用 API Key、Base URL 和组织
信息；本地模型使用 Ollama Host、端口和 GPU 开关。两者共享模型名、温度、最大 Token、
超时和最大重试次数。

`Translator` 支持启用开关、Provider 类型、请求超时、扩展参数，以及 Google、百度、阿里云
和火山引擎的 Provider 配置。Google 还支持 API 版本、API Key、项目、区域和 parent 资源
路径；百度支持 App ID 和密钥；阿里云支持访问密钥和地域；火山引擎支持访问密钥和地域。

`AppInfo` 用于描述项目、服务、实例、版本、主机、端点、启动时间、环境、区域、可用区、
标签、构建信息和自定义元数据。

## 使用生成代码

```go
package example

import configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"

cfg := &configv1.Bootstrap{
	Server: &configv1.Server{
		Http: &configv1.Server_Http{Addr: ":8000"},
	},
}
```

`Bootstrap` 当前的顶层字段为：`server`、`client`、`data`、`trace`、`logger`、
`registry`、`config`、`oss`、`notify`、`authn`、`authz`、`pprof`、`ai`、`oauth`、
`translator` 和 `mfa`。`AppInfo` 是独立的运行时元数据消息，不嵌入 `Bootstrap`。

AI 使用 `configv1.AI`，其 `AI.Model` 与上游 `kratos-bootstrap` 的模型配置保持一致；
旧的 `Client.Llm` 配置已移除。OAuth 使用独立的 `configv1.OAuth`，按 `providers`
映射创建 Provider。翻译使用 `configv1.Translator`，按 `type` 选择厂商。密钥配置使用
独立的 `configv1.Key`，不放入 `Bootstrap`。

## 依赖与工具

在仓库根目录执行以下命令可安装生成器和 CLI：

```bash
make plugin
make cli
```

运行生成需确保 `buf` 在 `PATH` 中；`buf.gen.yaml` 当前使用以下本地插件：

- `protoc-gen-go`
- `protoc-gen-go-grpc`
- `protoc-gen-go-http`
- `protoc-gen-go-errors`

`buf.yaml` 当前依赖 `googleapis`、`protoc-gen-validate`、`kratos/apis`、`gnostic` 和
`gogo/protobuf`。这些依赖由 Buf 管理，不要将外部 proto 复制到本目录。当前契约没有
RPC Service 定义，生成结果主要是各消息的 `*.pb.go` 文件。

## 生成代码

推荐在仓库根目录执行：

```bash
make api
```

该命令等价于：

```bash
cd api
buf generate
```

`buf.gen.yaml` 的关键行为：

- `clean: true`：生成前清理输出目录中的旧产物。
- `managed.enabled: true`：由 Buf 统一管理文件选项。
- `go_package_prefix: github.com/liujitcn/kratos-kit/api/gen/go`。
- 所有插件输出到 `api/gen/go`，并使用 `paths=source_relative`。

修改 `api/buf.yaml` 的 `deps` 或需要升级远程依赖时执行：

```bash
cd api
buf dep update
```

`buf.lock` 由 Buf 自动维护，更新依赖后应与 proto 改动一并提交。

## 校验

在 `api` 目录执行：

```bash
buf lint
buf build
buf generate
go test ./...
```

其中 `buf build` 用于检查依赖和描述符是否可构建，`go test ./...` 用于验证生成的 Go
包。若只修改了文档，可跳过生成；修改 proto 后应按上述顺序重新生成并检查差异。

当前 proto 保留部分历史枚举值名称（`HTTP`、`SSE`、`STDIO`、`IN_PROCESS`、
`CLOUD_MODEL`、`LOCAL_MODEL`），因此 `buf lint` 会报告枚举前缀命名问题。除非同步
规划 API 变更，不要直接重命名这些值；重命名会影响生成的 Go 常量和下游配置。

## Proto 引用约定

Buf 模块根目录是 `api/proto`。同一模块内使用模块相对路径引用，例如：

```proto
import "config/v1/tls.proto";
```

外部项目使用生成包：

```go
import configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
```

## 密钥与敏感配置

`configv1.Key` 的配置文件内容直接对应 `key.yaml`。它只声明 Provider 和非敏感启动参数，
根密钥及 Provider 认证信息必须通过 Secret Manager、Kubernetes Secret、环境变量或工作负载
身份提供，不应直接写入 Proto 配置或提交到仓库。

## 发布引用

`api` 子模块承载公共 proto 契约和生成代码。配置契约或下游依赖基线同步发布时，外部项目
应优先锁定最新 `api/v*` tag，确保 `config/v1` 包路径以及 HTTP、gRPC、MCP、SSE 配置
字段与生成代码保持一致。
