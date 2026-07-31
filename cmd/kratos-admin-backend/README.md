# kratos-admin-backend

`kratos-admin-backend` 用于创建基于
`github.com/liujitcn/kratos-admin/backend/core` 的空业务后端项目。生成项目沿用
`kratos-admin/backend` 的 Proto、biz、service、server、data、migration 和
projectdocs 分层，但不预置业务接口或数据库模型。

生成项目同时提供 `NewModule` 和 `NewApp` 两条 Wire 装配入口，分别用于挂载到
其他 Core 宿主和独立启动。

生成结果不包含独立的 `internal/metadata` 项目元数据文件。启动入口直接设置
`AppInfo`，项目文档以及后续接入的 OpenAPI/Swagger 文档统一使用其中的
`Project` 和 `Name`。

## 安装

```bash
go install github.com/liujitcn/kratos-kit/cmd/kratos-admin-backend@latest
```

生成 projectdocs 需要同时安装：

```bash
go install github.com/liujitcn/kratos-kit/cmd/project-docs@latest
```

## 使用

```bash
kratos-admin-backend create --module github.com/example/order
```

命令根据 module 最后一段创建 `./order`，拒绝覆盖已有目录。创建过程中会生成
projectdocs、整理 Go module、执行 Wire，并通过 `go test ./...` 验证生成项目；
任一步骤失败都会清理本次新建的不完整目录。
