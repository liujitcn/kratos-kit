# __PROJECT_NAME__

这是一个基于 `kratos-core` 和 `kratos-admin/backend` 的 Admin 后端服务。

## 已提供能力

- 登录、JWT、验证码、MFA 和 OAuth；
- 用户、租户、部门、岗位、角色、菜单和接口权限；
- 文件资产、消息通知、操作审计、登录日志和运行日志；
- OpenAPI、文档、国际化、Casbin 策略和数据库迁移同步；
- Admin 定时任务、SSE 流和队列消费者；
- 当前项目业务模块的 HTTP、gRPC、MCP、资源和任务扩展入口。

Admin 通过公开的 `github.com/liujitcn/kratos-admin/backend` ProviderSet 接入，生成项目不
依赖 Admin 的 `internal` 包。

## 配置和启动

默认配置使用本地 MySQL、Redis 和内存队列。项目根目录提供 `docker-compose.yaml`：

```bash
cd ..
make infra-up
make run
```

数据库迁移会自动创建 Core/Admin 表，并执行 Admin 默认菜单、权限和开发数据初始化。
生产环境必须替换 `configs/auth.yaml` 中的 JWT 密钥以及 `configs/data.yaml` 中的连接信息。

## 扩展业务

在项目根包的 `bootstrap.go` 中，Admin 和当前项目模块通过 `hostProviderSet` 合并。新增
业务模块时只需增加自己的 ProviderSet、Resource、Module、Migration 和生成产物，不修改
Admin 依赖源码。

```bash
make gen
make test
make build
```
