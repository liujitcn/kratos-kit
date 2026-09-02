# __PROJECT_NAME__

`__PROJECT_NAME__` 是基于最新 `kratos-core` 的最小 Kratos 后端服务模板。

Go module：`__MODULE_PATH__`

## 目录

```text
.
├── api
│   ├── proto
│   └── gen/go
├── configs
├── data
├── scripts
│   └── docker-entrypoint.sh
├── internal
│   ├── cmd/server
│   ├── module
│   └── openapi
│       └── assets
├── bootstrap.go
├── Dockerfile
└── Makefile
```

服务入口位于 `internal/cmd/server`，项目根包提供 Core 依赖注入集合。`internal/module`
中预置了一个空业务模块，新增业务时可以在此注册 HTTP、gRPC 或 MCP 服务。

默认配置使用 SQLite 文件、内存缓存和内存队列，不需要预先启动 MySQL、Redis 或 Consul。
生产环境可以按 `kratos-core` 的配置约定替换数据库、缓存、队列和注册中心实现。

## 启动

```bash
make run
```

HTTP 默认监听 `:7001`，gRPC 默认监听 `:6001`。

## 生成与验证

```bash
make init
make gen
make test
make build
```

后端发布打包：

```bash
make package-binary
```

`api/gen/go`、OpenAPI 文件和 `internal/cmd/server/wire_gen.go` 都是生成产物，
应通过 `make gen` 或对应生成命令更新。
