# __PROJECT_NAME__

这是一个基于 `kratos-core` 和 `kratos-admin/backend` 的 Admin 后端服务。

## 已提供能力

- 登录、JWT、验证码、MFA 和 OAuth；
- 用户、租户、部门、岗位、角色、菜单和接口权限；
- 文件资产、消息通知、操作审计、登录日志和运行日志；
- OpenAPI、国际化、Casbin 策略和数据库迁移同步；
- Admin 定时任务、SSE 流和队列消费者；
- 当前项目业务模块的 HTTP、gRPC、MCP、资源和任务扩展入口。

业务根包通过 `github.com/liujitcn/kratos-admin/backend` ProviderSet 接入 Admin。
Admin 在自身包内完成数据适配器装配，生成项目只使用公共构造入口和 Core 接口，
不引用、复制或修改 Admin 的 `internal` 代码。

## 模块结构

后端只有一个 Go module：`__MODULE_PATH__`，业务代码和 `internal/cmd/server` 服务入口都属于
该模块。没有额外的宿主 `go.mod` 或 `go.work`，`go test ./...` 与 `make test` 都覆盖整个后端。

## 配置和启动

配置布局与 Admin 一致。先准备 MySQL、Redis、Consul 和 Vault，在环境提供 `VAULT_TOKEN`；
项目只生成通用配置，不复制上游私有的 `*.dev.yaml`。

```bash
make init
make run-only
```

数据库迁移会自动创建 Core/Admin 表，并执行 Admin 默认菜单、权限和开发数据初始化。
生产环境必须配置独立的 Vault 根密钥与数据库连接信息。

## 扩展业务

在项目根包的 `bootstrap.go` 中，Admin 和当前项目模块通过 `hostProviderSet` 合并。新增
业务模块时只需增加自己的 ProviderSet、Resource、Module、Migration 和生成产物，不修改
Admin 依赖源码。

Resource 的模型来自 `internal/data.Models()`，语言来自 `internal/i18n/assets`，OpenAPI 来自
`internal/openapi/assets`，SQL 位于 `migration/assets/v0.0.1/mysql`。这些入口均已注册，初始业务内容为空。
项目业务模块通过 `internal/module/wire.go` 装配，入口变化后执行 `make public-wire wire`。
`biz`、`service`、`server` 的业务目录及 `config`、`task`、`data` 默认包含 `init.go`，
各层 `ProviderSet` 已汇总到 `internal/module.ProviderSet`；新增构造函数时加入对应集合。
新增业务表后设置 `GORM_TABLE` 并在 `internal/data.Models()` 中汇总生成模型。

```bash
make gen
make test
make build
```
