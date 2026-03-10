# api 模块说明

## 概述

`api` 模块用于维护 protobuf 定义并生成 Go 代码。

- proto 源码目录：`api/protos`
- 生成代码目录：`api/gen/go`
- buf 配置：`api/buf.yaml`、`api/buf.gen.yaml`、`api/buf.lock`

## 目录结构

```text
api/
├── protos/             # proto 定义
│   └── conf/
├── gen/go/             # 生成结果（*.pb.go）
├── buf.yaml            # buf 模块与依赖配置
├── buf.gen.yaml        # 代码生成插件配置
└── buf.lock            # 依赖锁文件（自动生成）
```

## 依赖与工具

在仓库根目录执行：

```bash
make plugin
make cli
```

至少需要以下命令可用：

- `buf`
- `protoc-gen-go`
- `protoc-gen-go-grpc`
- `protoc-gen-go-http`
- `protoc-gen-go-errors`

## 生成代码

在仓库根目录执行：

```bash
make api
```

等价命令：

```bash
cd api
buf generate
```

`buf.gen.yaml` 已配置：

- `managed.enabled: true`
- `go_package_prefix: github.com/liujitcn/kratos-kit/api/gen/go`
- 输出路径：`gen/go`
- 生成插件：`go`、`go-grpc`、`go-http`、`go-errors`

## 更新 `buf.lock`

当你修改了 `api/buf.yaml` 中的 `deps`，或希望升级远程依赖版本时，执行：

```bash
cd api
buf dep update
```

说明：

- `buf.lock` 由 buf 自动维护，不要手动编辑。
- 更新后请一并提交 `buf.lock`。

## 校验建议

生成前后建议执行：

```bash
cd api
buf lint
buf build
buf generate
```

回到仓库根目录执行：

```bash
go test ./...
```

## Proto 引用约定

`buf` 模块根是 `api/protos`，因此在 proto 中引用同模块文件时，按模块内相对路径写 import，例如：

```proto
import "conf/tls.proto";
```
