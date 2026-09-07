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

生成项目使用 MySQL、Redis 和内存队列，并生成 `docker-compose.yaml`；进入项目后可执行
`make infra-up` 准备本地依赖。随后执行 `make init` 安装后端工具和前端依赖，再按项目
README 中的命令开发、生成和构建。
