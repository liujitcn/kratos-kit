# kratos-admin

`kratos-admin` 用于创建包含前后端的完整项目。项目根目录和后端由本命令内置模板
生成，前端通过三个上游 CLI 生成，不把前端源码复制进 Go 模板。

## 安装

```bash
go install github.com/liujitcn/kratos-kit/cmd/kratos-admin@latest
```

## 使用

传入项目名即可创建携带 Admin 登录、用户、角色、菜单、权限、文件、日志、任务和消息能力的完整项目：

```bash
kratos-admin create shop-admin
```

命令会创建以下结构：

```text
shop-admin
├── backend       # Core + Admin + 当前项目业务模块
├── frontend
│   ├── admin     # @liujitcn/kratos-admin-cli
│   ├── uni-app   # @liujitcn/kratos-uni-app-cli
│   └── taro-app  # @liujitcn/kratos-taro-app-cli
├── scripts       # 项目级前后端快捷脚本
├── backend/scripts
├── frontend/Makefile
├── frontend/scripts
├── Makefile
└── README.md
```

后端 Go module 默认是 `github.com/example/<project>/backend`，可以显式指定；前端
默认创建 `app` 业务 module：

```bash
kratos-admin create shop-admin \
  --module github.com/acme/shop-admin/backend \
  --frontend-module shop
```

生成过程会依次调用以下 CLI，并迁移后端、前端各自的 Makefile 与脚本入口：

- `@liujitcn/kratos-admin-cli@latest`
- `@liujitcn/kratos-uni-app-cli@latest`
- `@liujitcn/kratos-taro-app-cli@latest`

每次生成都会通过 pnpm 命令行参数将 `@liujitcn` 作用域临时指向 npm 官方源，并设置
`dlx-cache-max-age=0`，避免镜像同步延迟或 dlx 旧缓存导致执行旧版 CLI。无需手动清理
缓存或设置环境变量；其他作用域沿用原有源，用户的全局及项目 pnpm 配置不会被修改。

生成过程会在后端初始化时执行 `go get github.com/liujitcn/kratos-admin/backend@latest`，
并使用 Backend 模块自身 `go.mod` 声明的 Admin API 版本，避免强制覆盖依赖导致跨版本组合。
随后执行后端 `go mod tidy`、Wire 和 `go test ./...`。任一前端 CLI 或后端初始化失败，
本命令都会清理本次新建的不完整项目目录。

### 后端模块边界

后端只有 `backend/go.mod` 一个模块，保留用户指定的 module 名；服务入口
`backend/internal/cmd/server` 属于同一模块，不生成额外的宿主 `go.mod`、`go.work` 或本地替换。

Admin 的公开 `adapter/core` 和 `adapter/kit` 构造函数统一接收数据库客户端，在函数内部初始化
所需仓储。Wire 只使用公开适配器和 Core/Kit 接口，生成项目不会引用 Admin 的 `internal` 包，
也不复制依赖源码。脱敏策略通过实例和请求上下文传递，不依赖全局默认解析器。
正式生成前须按 Kit redact/server-grpc、Core、Admin Backend 的依赖顺序发布修复版本；安装新版生成器不会修复旧版依赖。

在 Backend 目录执行 `go test ./...` 或 `make test` 会同时覆盖业务代码和服务入口，构建使用
`make build`。初始化时设置 `GOWORK=off`，避免继承调用方的 Go workspace。

生成项目使用 MySQL、Redis 和内存队列，并生成 `docker-compose.yaml`；进入项目后可执行
`make infra-up` 准备本地依赖。随后执行 `make init` 安装后端工具和前端依赖，再按项目
README 中的命令开发、生成和构建。

## 验证生成器

```bash
GOWORK=off go test ./...
GOWORK=off KRATOS_ADMIN_INTEGRATION=1 go test -run TestGeneratedBackendBuilds -v
```

集成测试需要 Go、Make 和 Wire，跳过前端 CLI，真实生成默认及自定义 `/v2` module 的后端，
执行 Wire、`go test ./...` 和生成项目的 `make test build`，并检查 Wire 产物不引用 Admin 内部包。
依赖解析沿用当前 Go 代理设置。测试未发布的跨仓库修复时，可额外设置
`KRATOS_ADMIN_BACKEND_DIR`、`KRATOS_CORE_DIR`、`KRATOS_KIT_REDACT_DIR` 和 `KRATOS_KIT_GRPC_DIR` 指向对应本地模块；
这些替换仅作用于测试的临时项目。
