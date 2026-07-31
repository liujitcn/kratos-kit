# __PROJECT_NAME__

`__PROJECT_NAME__` 是基于
`github.com/liujitcn/kratos-admin/backend/core` 创建的空业务 Kratos 后端项目。

Go module：

```text
__MODULE_PATH__
```

## 目录

```text
.
├── api
│   ├── proto
│   └── gen/go
├── configs
├── data
│   ├── admin/assets
│   ├── app/assets
│   └── logs
├── docs
├── internal
│   ├── biz
│   ├── cmd/server
│   ├── config
│   ├── const
│   ├── data
│   ├── projectdocs
│   ├── server
│   └── service
├── migration/assets
├── projectdoc
├── app.go
├── wire.go
└── wire_gen.go
```

模板不预置业务 Proto、数据库模型或业务接口。新增能力时按
`Proto -> biz -> service -> server 注册` 的顺序实现，并通过项目命令生成协议和
Wire 产物。

项目根包同时提供两种运行形态：

- `NewModule`：返回可挂载到其他 Core 宿主的 `Runtime`。
- `NewApp`：复用同一模块依赖创建独立 HTTP/gRPC 应用。

## 启动

首次开发前安装生成工具：

```bash
make init
```

启动 HTTP `:7001` 和 gRPC `:6001`：

```bash
make run
```

也可以直接运行：

```bash
go run ./internal/cmd/server --conf ./configs
```

## 生成与验证

```bash
make gen
make test
make build
```

`api/gen/go`、`internal/cmd/server/assets/openapi.yaml`、
`internal/projectdocs/assets/catalog.json`、`internal/projectdocs/catalog_gen.go`
和 `wire_gen.go` 都是生成产物，不应手工修改；项目文档 JSON 按项目和文件目录保存
为递归树。项目文档和后续业务模块提供的 OpenAPI/Swagger 文档都应使用启动入口
`AppInfo` 的 `Project` 和 `Name` 作为项目标识与展示名称。
