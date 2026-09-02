# kratos-admin

`kratos-admin` 用于创建包含前后端的完整项目。项目根目录和后端由本命令内置模板
生成，前端通过三个上游 CLI 生成，不把前端源码复制进 Go 模板。

## 安装

```bash
go install github.com/liujitcn/kratos-kit/cmd/kratos-admin@latest
```

## 使用

只传项目名即可创建默认项目：

```bash
kratos-admin create shop-admin
```

命令会创建以下结构：

```text
shop-admin
├── backend       # 基于 kratos-core 的最小 Kratos 服务
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

随后执行后端 `go mod tidy`、Wire 和 `go test ./...`。任一前端 CLI 或后端初始化
失败，本命令都会清理本次新建的不完整项目目录。

生成项目默认使用 SQLite 文件、内存缓存和内存队列，不需要预先启动 MySQL、Redis
或 Consul。进入项目后执行 `make init` 安装后端工具和前端依赖，再按根目录
README 中的命令开发、生成和构建。
