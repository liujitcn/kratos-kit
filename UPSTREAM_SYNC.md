# go-wind-plugins 同步最终说明

本文记录 `kratos-kit` 对
[`tx7do/go-wind-plugins`](https://github.com/tx7do/go-wind-plugins)
的最终源码同步结果。对比基线为上游提交
[`d8a493a75f72939eca7f812efa486406db4a4454`](https://github.com/tx7do/go-wind-plugins/commit/d8a493a75f72939eca7f812efa486406db4a4454)，
框架基线为 `github.com/go-kratos/kratos/v3 v3.0.0`。

## 同步原则

1. Kratos v3 core/contrib 已覆盖的能力直接复用，不维护第二套接口和生命周期。
2. Kratos 未覆盖的能力直接实现其 `Source`、`Codec`、`Middleware`、`Limiter`、
   `transport.Server` 或 `slog.Handler` 契约。
3. 上游存在并发、生命周期或协议问题时，只同步能力并修正实现。
4. 浅层 SDK 转发、重复算法和不完整实现不引入。

## 最终新增

| 领域 | 新增代码 | 原因 |
| --- | --- | --- |
| Config | `config/{fs,http,oss,redis,vault,zookeeper}` | Kratos 不提供这些 Source；直接实现 `config.Source`/`Watcher`，并修正重复首帧、无 ETag 比较和 ZooKeeper 节点重建问题。 |
| Encoding | `encoding/{avro,bson,cbor,flatbuffers,gob,thrift,toml}` | Kratos 不提供这些格式；直接实现并注册 Kratos Codec，保留 schema/IDL 生成类型约束。 |
| AuthN | `auth/authn/engine/{apikey,basicauth,hmac,mtls,oauth2,oidc,session}` | Kratos 没有这些 provider；每个实现只暴露真实具备的认证能力。 |
| AuthZ | `auth/authz/engine/{cerbos,opa,zanzibar}` | 补充策略引擎和 OpenFGA/Keto 等关系鉴权适配端口。 |
| Broker | `broker/server.go`、`broker/nats` | 统一 broker 的 Kratos 生命周期，并支持 Core NATS、JetStream、队列订阅、请求响应和 tracing。 |
| Cache | `cache/store`、`cache/local`、`cache/redis/store.go` | 提供 context 化 Store、泛型本地缓存和 Redis Store。 |
| 容错 | `circuitbreaker`、`ratelimit`、`retry` | 补充请求级熔断令牌、Sentinel/令牌桶限流，以及支持 context、退避和抖动的重试器。 |
| 可观测性 | `health`、`metrics`、`logger/multi_handler.go` | 增加 readiness 聚合、Prometheus/OTLP/Datadog provider 和 slog 多路分发。 |
| OSS | `oss/s3` | 使用 AWS SDK v2 提供 S3-compatible 对象存储，并兼容现有 `oss.OSS`。 |
| API 文档 | `swagger-ui/redoc.go` | 在现有 module 内增加 ReDoc，共用 OpenAPI 来源、鉴权和 Kratos HTTP Server。 |
| SSE | `transport/sse/auth.go` | 为命名流订阅补充令牌提取和授权扩展点。 |

## 最终修改

| 范围 | 修改与原因 |
| --- | --- |
| API/RPC | 客户端增加 circuit breaker 配置；服务端同名字段废弃；补齐 CORS、S3、gRPC CustomHealth；HTTP/gRPC 共享客户端中间件装配。 |
| Request ID | 使用 Kratos transport Header 完成服务端读写和 HTTP/gRPC 客户端透传；Kratos 只提供 Header 抽象，不负责生成 ID。 |
| Auth | 将大接口拆成请求认证、令牌认证、凭证注入、身份签发和关闭等窄接口；Casbin 增加并发保护并把非法策略改为返回错误。 |
| Cache/OAuth | `Cache` 增加原子 `GetDel`；Redis 使用 `GETDEL`，内存实现持锁读取删除；OAuth state 改为一次性消费，消除 `GET` 后 `DEL` 的并发窗口。 |
| Cron | 增加启动前注册、移除/查询、时区、logger、panic recovery 和幂等停止；使用 `cron://scheduler` 虚拟端点。 |
| SSE | 增加事件元数据、`Last-Event-ID` 重放、CORS、认证、流管理和请求快照，并修正关闭竞态、重复流和锁内回调。 |
| Tracing | OTLP endpoint 为空时使用 `localhost:4318` 或 `localhost:4317`，使实现与默认端口说明一致。 |

## 最终删除

| 删除内容 | 替代位置或原因 |
| --- | --- |
| `rpc/middleware/validate/validate.go` | Kratos 已在 [`middleware/validate/validate.go#L13-L57`](https://github.com/go-kratos/kratos/blob/v3.0.0/middleware/validate/validate.go#L13-L57) 提供 `Validate()`、`ValidatorFunc` 链和错误转换；ProtoValidate 保留为本地 validator。 |
| HTTP/gRPC 重复 recovery、metadata 和认证装配 | 收敛到 `rpc/client_middleware.go`，底层继续使用 Kratos recovery/metadata。 |
| 服务端空 circuit breaker 分支 | Kratos v3 仅提供 [`Client()`](https://github.com/go-kratos/kratos/blob/v3.0.0/middleware/circuitbreaker/circuitbreaker.go#L36-L68)，现由客户端配置启用。 |
| Cron 无效 context、私有 keepalive 和自建停止 channel | context 从未传给任务，keepalive 无构造入口；停止改用 `robfig/cron.Cron.Stop()` 返回的 context，健康服务复用应用现有 Kratos gRPC Server。 |
| SSE 空 `Server.run()` 和重复编码/广播代码 | 统一到现有 Server/StreamManager 实现，减少重复分支和锁占用。 |
| 全部 `*_test.go` | 按清理要求删除；因此发布验证以逐 module 构建为主。 |
| 五份阶段性审计报告 | 内容已收敛到本文，避免多个结论随代码变化而失真。 |

校验迁移后，请求对象自身 `Validate()` 先执行，ProtoValidate 后执行；旧本地实现顺序相反，
两类校验同时失败时优先返回的错误会变化。

## Kratos v3 去重位置

| 不再重复实现的能力 | Kratos v3.0.0 位置 |
| --- | --- |
| Config 聚合、env/file 和 contrib provider | [`config`](https://github.com/go-kratos/kratos/tree/v3.0.0/config)、[`contrib/config`](https://github.com/go-kratos/kratos/tree/v3.0.0/contrib/config) |
| Codec 核心、JSON/Proto/XML/YAML/Msgpack | [`encoding`](https://github.com/go-kratos/kratos/tree/v3.0.0/encoding)、[`contrib/encoding/msgpack`](https://github.com/go-kratos/kratos/tree/v3.0.0/contrib/encoding/msgpack) |
| Errors | [`errors/errors.go#L22-L152`](https://github.com/go-kratos/kratos/blob/v3.0.0/errors/errors.go#L22-L152) |
| Registry 和 contrib provider | [`registry`](https://github.com/go-kratos/kratos/tree/v3.0.0/registry)、[`contrib/registry`](https://github.com/go-kratos/kratos/tree/v3.0.0/contrib/registry) |
| slog 日志核心和过滤 | [`log`](https://github.com/go-kratos/kratos/tree/v3.0.0/log) |
| 默认 SRE circuit breaker 和 BBR rate limiter | [`internal/circuitbreaker`](https://github.com/go-kratos/kratos/tree/v3.0.0/internal/circuitbreaker)、[`internal/ratelimit`](https://github.com/go-kratos/kratos/tree/v3.0.0/internal/ratelimit) |
| HTTP/gRPC 基础 transport 和生命周期 | [`transport`](https://github.com/go-kratos/kratos/tree/v3.0.0/transport) |

上游 `log/{charm,glog,hclog,phuslu}` 等浅包装、存在生命周期问题的远程日志 sink、重复的
Redis broker、空事务接口、重复 API Key/RBAC 模型及不完整 Cedar/AWS IAM 实现均不采用。

## 发布顺序

仓库是多 module 工作区。发布时必须从无内部依赖的底层 module 开始，逐层执行：升级已发布
的自有依赖、`GOWORK=off go mod tidy`、测试、提交、推送和打 tag。本轮发布过程中用于
`metrics` 和 `oss/s3` 联调的临时 `replace`，已在底层正式 tag 发布并更新上层依赖后从
`go.work` 移除。
