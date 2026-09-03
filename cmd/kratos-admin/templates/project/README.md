# __PROJECT_NAME__

`__PROJECT_NAME__` 是由 `kratos-admin` 创建的完整 Admin 项目，包含：

- `kratos-core` 运行时；
- `kratos-admin/backend` 提供的登录、用户、角色、菜单、权限、文件、日志、任务和消息能力；
- 当前项目自己的业务模块扩展入口；
- 管理端、uni-app 和 Taro 三套前端 workspace。

## 目录

```text
.
├── backend       # Core + Admin + 当前项目业务模块
├── frontend
│   ├── admin     # 管理端 CLI 生成
│   ├── uni-app   # uni-app CLI 生成
│   └── taro-app  # Taro CLI 生成
├── docker-compose.yaml
├── scripts
├── Makefile
└── README.md
```

前端源码由上游 CLI 生成，后端通过公开的 `kratos-admin/backend` ProviderSet 接入 Admin，
不复制 Admin 的 `internal` 代码。

## 启动

默认配置使用本地 MySQL、Redis 和内存队列。先准备本地基础设施：

```bash
make infra-up
make run
```

默认开发账号和菜单权限由 Admin 迁移初始化。数据库连接、Redis、JWT 和跨域配置位于
`backend/configs`，生产环境必须替换示例密钥和连接信息。

管理端前端：

```bash
make frontend-dev
```

## 扩展业务模块

新增业务时，按以下边界组织代码：

```text
backend/api/proto/<domain>
backend/internal/biz/<domain>
backend/internal/data/<domain>
backend/internal/service/<domain>
backend/internal/server/<domain>
backend/internal/task/<domain>
backend/internal/module
backend/migration
```

业务模块只提供自己的 Service、Resource、Migration 和任务，通过 `backend/bootstrap.go`
中的宿主 ProviderSet 与 Admin 并列组合。接口、OpenAPI、Wire 和前端 RPC 使用项目 Makefile
中的生成命令完成。

```bash
make -C backend gen
make -C frontend ts
make test
```
