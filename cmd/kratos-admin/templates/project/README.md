# __PROJECT_NAME__

`__PROJECT_NAME__` 是由 `kratos-admin` 创建的完整 Admin 项目，包含：

- `kratos-core` 运行时；
- `kratos-admin/backend` 提供的登录、用户、角色、菜单、权限、文件、日志、任务和消息能力；
- 当前项目自己的业务模块扩展入口；
- 管理端、uni-app 和 Taro 三套前端 workspace。

## 目录

```text
.
├── backend
│   ├── api/proto
│   ├── internal/{biz,data,service,server,task,module}
│   ├── internal/{i18n,openapi}/assets
│   ├── migration/assets/v0.0.1/mysql
│   ├── configs
│   ├── scripts
│   └── Makefile
├── frontend
│   ├── admin     # 管理端 CLI 生成
│   ├── uni-app   # uni-app CLI 生成
│   └── taro-app  # Taro CLI 生成
├── scripts
├── Makefile
└── README.md
```

前端源码、语言注册和检查构建工具全部由三端 npm CLI 生成；后端通过公开的 `kratos-admin/backend` ProviderSet 接入 Admin，
不复制 Admin 的 `internal` 代码。
后端业务代码和服务入口属于同一个 Go module，`make -C backend test` 覆盖整个后端。
前端保持 `apps/<terminal>` 宿主与 `packages/modules/<module>` 业务模块布局，业务模块清单为 `__MODULES__`；
Core/System 底座通过 npm 依赖复用，本项目只检查和打包自己的业务模块。

## 启动

新目录尚未初始化 Git 时先执行 `git init`，再用根目录 `make init` 安装 Git hooks。

先准备配置所需的 MySQL、Redis、Consul 和已解封的 Vault，并在运行环境提供有根密钥读取权限的
`VAULT_TOKEN`。生成器不会创建数据库服务或复制 Admin 私有的 `*.dev.yaml` 配置。

```bash
make -C backend init
make -C frontend init
make -C backend run-only
```

默认开发账号和菜单权限由 Admin 迁移初始化。数据库连接、Redis、JWT 和跨域配置位于
`backend/configs`，生产环境必须替换示例密钥和连接信息。

管理端前端：

```bash
make -C frontend run-admin
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
backend/internal/i18n/assets
backend/internal/openapi/assets
backend/migration/assets/v0.0.1/mysql
```

业务模块只提供自己的 Service、Resource、Migration 和任务，通过 `backend/bootstrap.go`
中的宿主 ProviderSet 与 Admin 并列组合。接口、OpenAPI、Wire 和前端 RPC 使用项目 Makefile
中的生成命令完成。

```bash
make -C backend gen
make -C frontend ts
make -C backend test
make -C frontend check
```

`Models`、`I18n`、`OpenAPI` 和 `Migrations` 已全部接入 Resource，初始模型、语言和业务 SQL 可以为空。
迁移资源在 Admin 之后执行。空目录通过占位文件保留；没有业务表或 Proto 时，对应生成目标明确跳过。
新增表后设置 `GORM_TABLE`，并在 `backend/internal/data/Models` 中汇总生成模型。

根目录负责 `gen/check/build/package/i18n/docker-*`；Go/Proto/Wire 位于后端 Makefile，三端 RPC、
前端检查与构建位于前端 Makefile。H5 构建统一输出到 `backend/data/{admin,uni-app,taro-app}`。
